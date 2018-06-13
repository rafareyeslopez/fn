package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/url"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/fnproject/fn_go/client/call"
	"github.com/fnproject/fn_go/client/operations"
	"github.com/fnproject/fn_go/models"
)

func callAsync(ctx context.Context, t *testing.T, u url.URL, content io.Reader) string {
	output := &bytes.Buffer{}
	_, err := callFN(ctx, u.String(), content, output, "POST", []string{})
	if err != nil {
		t.Errorf("Got unexpected error: %v", err)
	}

	expectedOutput := "call_id"
	if !strings.Contains(output.String(), expectedOutput) {
		t.Errorf("Assertion error.\n\tExpected: %v\n\tActual: %v", expectedOutput, output.String())
	}

	type CallID struct {
		CallID string `json:"call_id"`
	}

	callID := &CallID{}
	json.NewDecoder(output).Decode(callID)

	if callID.CallID == "" {
		t.Errorf("`call_id` not suppose to be empty string")
	}
	t.Logf("Async execution call ID: %v", callID.CallID)
	return callID.CallID
}

func callSync(ctx context.Context, t *testing.T, u url.URL, content io.Reader) string {
	output := &bytes.Buffer{}
	resp, err := callFN(ctx, u.String(), content, output, "POST", []string{})
	if err != nil {
		t.Errorf("Got unexpected error: %v", err)
	}

	callID := resp.Header.Get("FN_CALL_ID")
	if callID == "" {
		t.Errorf("Assertion error.\n\tExpected call id header in response, got: %v", resp.Header)
	}

	t.Logf("Sync execution call ID: %v", callID)
	return callID
}

func TestCanCallfunction(t *testing.T) {
	t.Parallel()
	s := setupHarness()
	defer s.Cleanup()

	s.GivenAppExists(t, &models.App{Name: s.AppName})
	rt := s.BasicRoute()
	rt.Type = "sync"
	s.GivenRouteExists(t, s.AppName, rt)

	u := url.URL{
		Scheme: "http",
		Host:   host(),
	}
	u.Path = path.Join(u.Path, "r", s.AppName, s.RoutePath)

	content := &bytes.Buffer{}
	output := &bytes.Buffer{}
	_, err := callFN(s.Context, u.String(), content, output, "POST", []string{})
	if err != nil {
		t.Errorf("Got unexpected error: %v", err)
	}
	expectedOutput := "Hello World!\n"
	if !strings.Contains(expectedOutput, output.String()) {
		t.Errorf("Assertion error.\n\tExpected: %v\n\tActual: %v", expectedOutput, output.String())
	}
}

func TestCallOutputMatch(t *testing.T) {
	t.Parallel()
	s := setupHarness()
	s.GivenAppExists(t, &models.App{Name: s.AppName})
	rt := s.BasicRoute()
	rt.Type = "sync"
	s.GivenRouteExists(t, s.AppName, rt)

	u := url.URL{
		Scheme: "http",
		Host:   host(),
	}
	u.Path = path.Join(u.Path, "r", s.AppName, s.RoutePath)

	content := &bytes.Buffer{}
	json.NewEncoder(content).Encode(struct {
		Name string
	}{Name: "John"})
	output := &bytes.Buffer{}
	_, err := callFN(s.Context, u.String(), content, output, "POST", []string{})
	if err != nil {
		t.Errorf("Got unexpected error: %v", err)
	}
	expectedOutput := "Hello John!\n"
	if !strings.Contains(expectedOutput, output.String()) {
		t.Errorf("Assertion error.\n\tExpected: %v\n\tActual: %v", expectedOutput, output.String())
	}
}

func TestCanCallAsync(t *testing.T) {
	newRouteType := "async"
	t.Parallel()

	s := setupHarness()
	s.GivenAppExists(t, &models.App{Name: s.AppName})
	rt := s.BasicRoute()
	rt.Type = "sync"
	s.GivenRouteExists(t, s.AppName, rt)

	u := url.URL{
		Scheme: "http",
		Host:   host(),
	}
	u.Path = path.Join(u.Path, "r", s.AppName, s.RoutePath)

	s.GivenRoutePatched(t, s.AppName, s.RoutePath, &models.Route{
		Type: newRouteType,
	})

	callAsync(s.Context, t, u, &bytes.Buffer{})
}

func TestCanGetAsyncState(t *testing.T) {
	newRouteType := "async"
	t.Parallel()
	s := setupHarness()

	s.GivenAppExists(t, &models.App{Name: s.AppName})
	rt := s.BasicRoute()
	rt.Type = "sync"
	s.GivenRouteExists(t, s.AppName, rt)

	u := url.URL{
		Scheme: "http",
		Host:   host(),
	}
	u.Path = path.Join(u.Path, "r", s.AppName, s.RoutePath)

	s.GivenRoutePatched(t, s.AppName, rt.Path, &models.Route{
		Type: newRouteType,
	})

	callID := callAsync(s.Context, t, u, &bytes.Buffer{})
	cfg := &call.GetAppsAppCallsCallParams{
		Call:    callID,
		App:     s.AppName,
		Context: s.Context,
	}
	cfg.WithTimeout(time.Second * 60)

	retryErr := apiCallWithRetry(t, 10, time.Second*2, func() (err error) {
		_, err = s.Client.Call.GetAppsAppCallsCall(cfg)
		return err
	})

	if retryErr != nil {
		t.Error(retryErr.Error())
	} else {
		callResponse, err := s.Client.Call.GetAppsAppCallsCall(cfg)
		if err != nil {
			switch err.(type) {
			case *call.GetAppsAppCallsCallNotFound:
				msg := err.(*call.GetAppsAppCallsCallNotFound).Payload.Error.Message
				t.Errorf("Unexpected error occurred: %v.", msg)
			}
		}
		callObject := callResponse.Payload.Call

		if callObject.ID != callID {
			t.Errorf("Call object ID mismatch.\n\tExpected: %v\n\tActual:%v", callID, callObject.ID)
		}
		if callObject.Path != s.RoutePath {
			t.Errorf("Call object route path mismatch.\n\tExpected: %v\n\tActual:%v", s.RoutePath, callObject.Path)
		}
		if callObject.Status != "success" {
			t.Errorf("Call object status mismatch.\n\tExpected: %v\n\tActual:%v", "success", callObject.Status)
		}
	}
}

func TestCanCauseTimeout(t *testing.T) {
	t.Parallel()
	s := setupHarness()
	defer s.Cleanup()

	s.GivenAppExists(t, &models.App{Name: s.AppName})

	rt := s.BasicRoute()
	timeout := int32(10)
	rt.Timeout = &timeout
	rt.Type = "sync"
	rt.Image = "funcy/timeout:0.0.1"
	s.GivenRouteExists(t, s.AppName, rt)

	u := url.URL{
		Scheme: "http",
		Host:   host(),
	}
	u.Path = path.Join(u.Path, "r", s.AppName, rt.Path)

	content := &bytes.Buffer{}
	json.NewEncoder(content).Encode(struct {
		Seconds int64 `json:"seconds"`
	}{Seconds: 11})
	output := &bytes.Buffer{}

	resp, _ := callFN(s.Context, u.String(), content, output, "POST", []string{})

	if !strings.Contains(output.String(), "Timed out") {
		t.Errorf("Must fail because of timeout, but got error message: %v", output.String())
	}
	cfg := &call.GetAppsAppCallsCallParams{
		Call:    resp.Header.Get("FN_CALL_ID"),
		App:     s.AppName,
		Context: s.Context,
	}
	cfg.WithTimeout(time.Second * 60)

	retryErr := apiCallWithRetry(t, 10, time.Second*2, func() (err error) {
		_, err = s.Client.Call.GetAppsAppCallsCall(cfg)
		return err
	})

	if retryErr != nil {
		t.Error(retryErr.Error())
	} else {
		callObj, err := s.Client.Call.GetAppsAppCallsCall(cfg)
		if err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
		if !strings.Contains("timeout", callObj.Payload.Call.Status) {
			t.Errorf("Call status mismatch.\n\tExpected: %v\n\tActual: %v",
				"output", "callObj.Payload.Call.Status")
		}
	}
}

func TestCallResponseHeadersMatch(t *testing.T) {
	t.Parallel()
	s := setupHarness()
	defer s.Cleanup()

	s.GivenAppExists(t, &models.App{Name: s.AppName})
	rt := s.BasicRoute()
	rt.Image = "denismakogon/os.environ"
	rt.Type = "sync"
	s.GivenRouteExists(t, s.AppName, rt)

	u := url.URL{
		Scheme: "http",
		Host:   host(),
	}
	u.Path = path.Join(u.Path, "r", s.AppName, rt.Path)
	content := &bytes.Buffer{}
	output := &bytes.Buffer{}
	callFN(s.Context, u.String(), content, output, "POST",
		[]string{
			"ACCEPT: application/xml",
			"ACCEPT: application/json; q=0.2",
		})
	res := output.String()
	if !strings.Contains("application/xml, application/json; q=0.2", res) {
		t.Errorf("HEADER_ACCEPT='application/xml, application/json; q=0.2' "+
			"should be in output, have:%s\n", res)
	}
}

func TestCanWriteLogs(t *testing.T) {
	t.Parallel()
	s := setupHarness()
	defer s.Cleanup()

	rt := s.BasicRoute()
	rt.Path = "/log"
	rt.Image = "funcy/log:0.0.1"
	rt.Type = "sync"

	s.GivenAppExists(t, &models.App{Name: s.AppName})
	s.GivenRouteExists(t, s.AppName, rt)

	u := url.URL{
		Scheme: "http",
		Host:   host(),
	}
	u.Path = path.Join(u.Path, "r", s.AppName, rt.Path)
	content := &bytes.Buffer{}
	json.NewEncoder(content).Encode(struct {
		Size int
	}{Size: 20})

	callID := callSync(s.Context, t, u, content)

	cfg := &operations.GetAppsAppCallsCallLogParams{
		Call:    callID,
		App:     s.AppName,
		Context: s.Context,
	}

	// TODO this test is redundant we have 3 tests for this?
	retryErr := apiCallWithRetry(t, 10, time.Second*2, func() (err error) {
		_, err = s.Client.Operations.GetAppsAppCallsCallLog(cfg)
		return err
	})

	if retryErr != nil {
		t.Error(retryErr.Error())
	} else {
		_, err := s.Client.Operations.GetAppsAppCallsCallLog(cfg)
		if err != nil {
			t.Error(err.Error())
		}
	}
}

func TestOversizedLog(t *testing.T) {
	t.Parallel()
	s := setupHarness()
	defer s.Cleanup()

	rt := s.BasicRoute()
	rt.Path = "/log"
	rt.Image = "funcy/log:0.0.1"
	rt.Type = "sync"

	s.GivenAppExists(t, &models.App{Name: s.AppName})
	s.GivenRouteExists(t, s.AppName, rt)

	size := 1 * 1024 * 1024 * 1024
	u := url.URL{
		Scheme: "http",
		Host:   host(),
	}
	u.Path = path.Join(u.Path, "r", s.AppName, rt.Path)
	content := &bytes.Buffer{}
	json.NewEncoder(content).Encode(struct {
		Size int
	}{Size: size}) //exceeding log by 1 symbol

	callID := callSync(s.Context, t, u, content)

	cfg := &operations.GetAppsAppCallsCallLogParams{
		Call:    callID,
		App:     s.AppName,
		Context: s.Context,
	}

	retryErr := apiCallWithRetry(t, 10, time.Second*2, func() (err error) {
		_, err = s.Client.Operations.GetAppsAppCallsCallLog(cfg)
		return err
	})

	if retryErr != nil {
		t.Error(retryErr.Error())
	} else {
		logObj, err := s.Client.Operations.GetAppsAppCallsCallLog(cfg)
		if err != nil {
			t.Error(err.Error())
		}
		log := logObj.Payload.Log.Log
		if len(log) >= size {
			t.Errorf("Log entry suppose to be truncated up to expected size %v, got %v",
				size/1024, len(log))
		}
	}
}

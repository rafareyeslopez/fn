package tests

import (
	"context"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/fnproject/fn_go/client"
	httptransport "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"
)

const lBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func getAPIURL() (string, *url.URL) {
	apiURL := getEnv("FN_API_URL", "http://localhost:8080")
	u, err := url.Parse(apiURL)
	if err != nil {
		log.Fatalf("Couldn't parse API URL: %s error: %s", apiURL, err)
	}
	return apiURL, u
}

func host() string {
	_, u := getAPIURL()
	return u.Host
}

func apiClient() *client.Fn {
	transport := httptransport.New(host(), "/v1", []string{"http"})
	if os.Getenv("FN_TOKEN") != "" {
		transport.DefaultAuthentication = httptransport.BearerToken(os.Getenv("FN_TOKEN"))
	}

	// create the API client, with the transport
	return client.New(transport, strfmt.Default)
}

func checkServer(ctx context.Context) error {
	if ctx.Err() != nil {
		log.Print("Server check failed, timeout")
		return ctx.Err()
	}

	apiURL, _ := getAPIURL()

	client := &http.Client{}
	req, err := http.NewRequest("GET", apiURL+"/version", nil)
	if err != nil {
		log.Panicf("Server check new request failed: %s", err)
	}

	req = req.WithContext(ctx)
	_, err = client.Do(req)
	if err != nil {
		log.Printf("Server is not up... err: %s", err)
		return err
	}
	return ctx.Err()
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

// testHarness provides context and pre-configured clients to an individual
// test, it has some helper functions to create Apps and Routes that mirror the
// underlying client operations and clean them up after the test is complete
// This is not goroutine safe and each test case should use its own harness.
type testHarness struct {
	Context      context.Context
	Cancel       func()
	Client       *client.Fn
	AppName      string
	RoutePath    string
	Image        string
	RouteType    string
	Format       string
	Memory       uint64
	Timeout      int32
	IdleTimeout  int32
	RouteConfig  map[string]string
	RouteHeaders map[string][]string

	createdApps map[string]bool
}

func randStringBytes(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = lBytes[rand.Intn(len(lBytes))]
	}
	return strings.ToLower(string(b))
}

// setupHarness creates a test harness for a test case - this picks up external options and
func setupHarness() *testHarness {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)

	ss := &testHarness{
		Context:      ctx,
		Cancel:       cancel,
		Client:       apiClient(),
		AppName:      "fnintegrationtestapp" + randStringBytes(10),
		RoutePath:    "/fnintegrationtestroute" + randStringBytes(10),
		Image:        "fnproject/hello",
		Format:       "default",
		RouteType:    "async",
		RouteConfig:  map[string]string{},
		RouteHeaders: map[string][]string{},
		Memory:       uint64(256),
		Timeout:      int32(30),
		IdleTimeout:  int32(30),
		createdApps:  make(map[string]bool),
	}
	return ss
}

func (s *testHarness) Cleanup() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	//for _,ar := range s.createdRoutes {
	//	deleteRoute(ctx, s.Client, ar.appName, ar.routeName)
	//}

	for app := range s.createdApps {
		safeDeleteApp(ctx, s.Client, app)
	}

	s.Cancel()
}

func envAsHeader(req *http.Request, selectedEnv []string) {
	detectedEnv := os.Environ()
	if len(selectedEnv) > 0 {
		detectedEnv = selectedEnv
	}

	for _, e := range detectedEnv {
		kv := strings.Split(e, "=")
		name := kv[0]
		req.Header.Set(name, os.Getenv(name))
	}
}

func callFN(ctx context.Context, u string, content io.Reader, output io.Writer, method string, env []string) (*http.Response, error) {
	if method == "" {
		if content == nil {
			method = "GET"
		} else {
			method = "POST"
		}
	}

	req, err := http.NewRequest(method, u, content)
	if err != nil {
		return nil, fmt.Errorf("error running route: %s", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(ctx)

	if len(env) > 0 {
		envAsHeader(req, env)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error running route: %s", err)
	}

	io.Copy(output, resp.Body)

	return resp, nil
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

func apiCallWithRetry(t *testing.T, attempts int, sleep time.Duration, callback func() error) (err error) {
	for i := 0; i < attempts; i++ {
		err = callback()
		if err == nil {
			t.Log("Exiting retry loop, API call was successful")
			return nil
		}
		t.Logf("[%v] - Retrying API call after unsuccessful attempt with error: %v", i, err.Error())
		time.Sleep(sleep)
	}
	return err
}

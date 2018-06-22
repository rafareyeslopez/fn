package models

import (
	"errors"
	"net/http"
	"time"
	"unicode"

	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/id"
)

type Trigger struct {
	ID          string          `json:"id" db:"id"`
	Name        string          `json:"name" db:"name"`
	AppID       string          `json:"app_id" db:"app_id"`
	FnID        string          `json:"fn_id" db:"fn_id"`
	CreatedAt   common.DateTime `json:"created_at,omitempty" db:"created_at"`
	UpdatedAt   common.DateTime `json:"updated_at,omitempty" db:"updated_at"`
	Type        string          `json:"type" db:"type"`
	Source      string          `json:"source" db:"source"`
	Annotations Annotations     `json:"annotations,omitempty" db:"annotations"`
}

func (t *Trigger) SetDefaults() {
	if time.Time(t.CreatedAt).IsZero() {
		t.CreatedAt = common.DateTime(time.Now())
	}
	if time.Time(t.UpdatedAt).IsZero() {
		t.UpdatedAt = common.DateTime(time.Now())
	}
	if t.ID == "" {
		t.ID = id.New().String()
	}
}

func (t1 *Trigger) Equals(t2 *Trigger) bool {
	eq := true
	eq = eq && t1.ID == t2.ID
	eq = eq && t1.Name == t2.Name
	eq = eq && t1.AppID == t2.AppID
	eq = eq && t1.FnID == t2.FnID

	eq = eq && t1.Type == t2.Type
	eq = eq && t1.Source == t2.Source
	eq = eq && t1.Annotations.Equals(t2.Annotations)

	// NOTE: datastore tests are not very fun to write with timestamp checks,
	// and these are not values the user may set so we kind of don't care.
	//eq = eq && time.Time(t1.CreatedAt).Equal(time.Time(t2.CreatedAt))
	//eq = eq && time.Time(t1.UpdatedAt).Equal(time.Time(t2.UpdatedAt))
	return eq
}

var triggerTypes = []string{"HTTP"}

func ValidTriggerTypes() []string {
	return triggerTypes
}

func validTriggerType(a string) bool {
	for _, b := range triggerTypes {
		if b == a {
			return true
		}
	}
	return false
}

var (
	ErrTriggerTypeUnknown = err{
		code:  http.StatusBadRequest,
		error: errors.New("Trigger Type Not Supported")}
	ErrTriggerMissingSource = err{
		code:  http.StatusBadRequest,
		error: errors.New("Missing Trigger Source")}
	ErrTriggerNotFound = err{
		code:  http.StatusNotFound,
		error: errors.New("Trigger not found")}
	ErrTriggerExists = err{
		code:  http.StatusConflict,
		error: errors.New("Trigger already exists")}
)

func (t *Trigger) Validate() error {
	if t.Name == "" {
		return ErrMissingName
	}

	if t.AppID == "" {
		return ErrMissingAppID
	}

	if t.FnID == "" {
		return ErrMissingFnID
	}

	if !validTriggerType(t.Type) {
		return ErrTriggerTypeUnknown
	}

	if t.Source == "" {
		return ErrTriggerMissingSource
	}

	err := t.Annotations.Validate()
	if err != nil {
		return err
	}

	return nil
}

func (t *Trigger) ValidCreate() error {

	if t.ID != "" {
		return ErrIDProvided
	}

	if !time.Time(t.CreatedAt).IsZero() {
		return ErrCreatedAtProvided
	}
	if !time.Time(t.UpdatedAt).IsZero() {
		return ErrUpdatedAtProvided
	}

	if t.Name == "" {
		return ErrMissingName
	}

	if len(t.Name) > maxTriggerName {
		return ErrTooLongName
	}
	for _, c := range t.Name {
		if !(unicode.IsLetter(c) || unicode.IsNumber(c) || c == '_' || c == '-') {
			return ErrInvalidName
		}
	}

	if t.AppID == "" {
		return ErrMissingAppID
	}

	if t.FnID == "" {
		return ErrMissingFnID
	}

	if !validTriggerType(t.Type) {
		return ErrTriggerTypeUnknown
	}

	if t.Source == "" {
		return ErrTriggerMissingSource
	}

	err := t.Annotations.Validate()
	if err != nil {
		return err
	}

	return nil
}

func (t *Trigger) Clone() *Trigger {
	clone := new(Trigger)
	*clone = *t // shallow copy

	if t.Annotations != nil {
		clone.Annotations = make(Annotations, len(t.Annotations))
		for k, v := range t.Annotations {
			// TODO technically, we need to deep copy the bytes
			clone.Annotations[k] = v
		}
	}
	return clone
}

func (t *Trigger) Update(patch *Trigger) {

	original := t.Clone()
	if patch.AppID != "" {
		t.AppID = patch.AppID
	}

	if patch.FnID != "" {
		t.FnID = patch.FnID
	}

	if patch.Name != "" {
		t.Name = patch.Name
	}

	if patch.Source != "" {
		t.Source = patch.Source
	}

	t.Annotations = t.Annotations.MergeChange(patch.Annotations)

	if !t.Equals(original) {
		t.UpdatedAt = common.DateTime(time.Now())
	}
}

type TriggerFilter struct {
	AppID string // this is exact match
	FnID  string // this is exact match

	Cursor  string
	PerPage int
}
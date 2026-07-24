package model_test

import (
	"errors"
	"fmt"
	"testing"

	"foundry/model"
)

func TestFailureClass_Retryable(t *testing.T) {
	tests := []struct {
		class model.FailureClass
		want  bool
	}{
		{model.FailureRateLimit, true},
		{model.FailureTemporaryUnavailable, true},
		{model.FailureTimeout, true},
		{model.FailureAuthentication, false},
		{model.FailureInvalidModel, false},
		{model.FailureUnsupportedCapability, false},
		{model.FailureClass("something_unrecognized"), false},
	}
	for _, tt := range tests {
		if got := tt.class.Retryable(); got != tt.want {
			t.Errorf("FailureClass(%q).Retryable() = %v, want %v", tt.class, got, tt.want)
		}
	}
}

func TestClassifyFailure_ExtractsClassFromFailureError(t *testing.T) {
	err := &model.FailureError{Class: model.FailureRateLimit, Err: errors.New("429 too many requests")}

	class, ok := model.ClassifyFailure(err)
	if !ok {
		t.Fatal("ClassifyFailure returned ok=false for a real FailureError")
	}
	if class != model.FailureRateLimit {
		t.Errorf("class = %q, want %q", class, model.FailureRateLimit)
	}
}

func TestClassifyFailure_UnclassifiedErrorReturnsFalse(t *testing.T) {
	_, ok := model.ClassifyFailure(errors.New("plain error"))
	if ok {
		t.Error("ClassifyFailure returned ok=true for a plain, unclassified error")
	}
}

func TestClassifyFailure_FindsFailureErrorThroughWrapping(t *testing.T) {
	inner := &model.FailureError{Class: model.FailureTimeout, Err: errors.New("context deadline exceeded")}
	wrapped := fmt.Errorf("engine: execute: %w", inner)

	class, ok := model.ClassifyFailure(wrapped)
	if !ok {
		t.Fatal("ClassifyFailure returned ok=false for a wrapped FailureError")
	}
	if class != model.FailureTimeout {
		t.Errorf("class = %q, want %q", class, model.FailureTimeout)
	}
}

func TestFailureError_ErrorAndUnwrap(t *testing.T) {
	inner := errors.New("401 unauthorized")
	fe := &model.FailureError{Class: model.FailureAuthentication, Err: inner}

	if fe.Error() != inner.Error() {
		t.Errorf("Error() = %q, want %q", fe.Error(), inner.Error())
	}
	if !errors.Is(fe, inner) {
		t.Error("errors.Is(fe, inner) = false, want true (Unwrap should expose inner)")
	}
}

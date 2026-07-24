package model

import "errors"

// FailureClass classifies why a generate Step's call to a model failed,
// deciding whether automatic model failover (ADR-0013, Proposed, sixth
// increment) may retry with the next Preferred model, or must fail the
// Step outright. Distinct from Status (this repository's own runtime
// health *state* of a model, reported independently of any one call) —
// FailureClass is about classifying one specific failed Execute call.
type FailureClass string

const (
	// FailureRateLimit: the vendor refused the call because a rate limit
	// was hit. Retryable.
	FailureRateLimit FailureClass = "rate_limit"
	// FailureTemporaryUnavailable: the vendor reported the model as
	// temporarily unavailable (e.g. an overloaded/5xx-class response).
	// Retryable.
	FailureTemporaryUnavailable FailureClass = "temporary_unavailable"
	// FailureTimeout: the call did not complete within its deadline.
	// Retryable.
	FailureTimeout FailureClass = "timeout"
	// FailureAuthentication: the credential used to call the model was
	// rejected. Never retryable — a different model behind the same
	// credential would fail identically, and one behind a different
	// credential is a configuration problem to fix, not paper over.
	FailureAuthentication FailureClass = "authentication"
	// FailureInvalidModel: the model ID itself was rejected (unknown to
	// the vendor, or unknown to this process's Model Registry). Never
	// retryable — trying the next Preferred entry doesn't fix a bad name.
	FailureInvalidModel FailureClass = "invalid_model"
	// FailureUnsupportedCapability: the call requested something this
	// model doesn't support (e.g. a tool-use request against a model with
	// no Capabilities.ToolUse). Never retryable — a different model may
	// well support it, but that is a capability-matching decision this
	// increment deliberately does not make (see the ADR's own Context).
	FailureUnsupportedCapability FailureClass = "unsupported_capability"
)

// Retryable reports whether a failure of this class permits automatic
// failover to the next Preferred model. True only for FailureRateLimit,
// FailureTemporaryUnavailable, and FailureTimeout — every other class,
// including any value not named above, defaults to false: failover is the
// exception a caller must explicitly classify into, never the default for
// an error this package doesn't recognize.
func (c FailureClass) Retryable() bool {
	switch c {
	case FailureRateLimit, FailureTemporaryUnavailable, FailureTimeout:
		return true
	default:
		return false
	}
}

// FailureError pairs a FailureClass with the underlying error, letting an
// Executor (or anything else observing a failed call) classify a failure
// for automatic model failover without every Executor needing a shared
// concrete error type — ClassifyFailure extracts it via errors.As. An
// Executor that never wraps its errors this way is simply never eligible
// for failover; nothing about its existing error-returning behavior needs
// to change.
type FailureError struct {
	Class FailureClass
	Err   error
}

func (e *FailureError) Error() string { return e.Err.Error() }
func (e *FailureError) Unwrap() error { return e.Err }

// ClassifyFailure extracts a FailureClass from err via errors.As (so it
// finds a FailureError anywhere in err's wrap chain, not only at the top),
// or reports ok=false if err was never classified this way.
func ClassifyFailure(err error) (FailureClass, bool) {
	var fe *FailureError
	if errors.As(err, &fe) {
		return fe.Class, true
	}
	return "", false
}

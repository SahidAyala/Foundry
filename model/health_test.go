package model_test

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/SahidAyala/Foundry/model"
)

func TestHealthManager_Get_UnreportedModelReturnsUnknown(t *testing.T) {
	h := model.NewHealthManager()

	got := h.Get("gpt-5.1")
	if got.Status != model.StatusUnknown {
		t.Errorf("Status = %q, want %q", got.Status, model.StatusUnknown)
	}
	if got.Reason != "" {
		t.Errorf("Reason = %q, want empty", got.Reason)
	}
	if !got.RetryAt.IsZero() {
		t.Errorf("RetryAt = %v, want the zero time", got.RetryAt)
	}
}

func TestHealthManager_ReportAndGet_RoundTrips(t *testing.T) {
	h := model.NewHealthManager()
	retryAt := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	want := model.Health{Status: model.StatusCooldown, Reason: "rate limited", RetryAt: retryAt}

	if err := h.Report("gpt-5.1", want); err != nil {
		t.Fatalf("Report failed: %v", err)
	}

	got := h.Get("gpt-5.1")
	if got != want {
		t.Errorf("Get(%q) = %+v, want %+v", "gpt-5.1", got, want)
	}
}

func TestHealthManager_Report_RefusesEmptyID(t *testing.T) {
	h := model.NewHealthManager()
	err := h.Report("", model.Health{Status: model.StatusAvailable})
	if err == nil {
		t.Fatal("Report with an empty ID returned nil error")
	}
}

func TestHealthManager_Report_OverwritesPreviousReport(t *testing.T) {
	h := model.NewHealthManager()
	if err := h.Report("gpt-5.1", model.Health{Status: model.StatusAvailable}); err != nil {
		t.Fatalf("first Report failed: %v", err)
	}
	if err := h.Report("gpt-5.1", model.Health{Status: model.StatusUnavailable, Reason: "401 unauthorized"}); err != nil {
		t.Fatalf("second Report failed: %v", err)
	}

	got := h.Get("gpt-5.1")
	if got.Status != model.StatusUnavailable {
		t.Errorf("Status = %q, want %q (the second, overwriting report)", got.Status, model.StatusUnavailable)
	}
	if !strings.Contains(got.Reason, "401") {
		t.Errorf("Reason = %q, want it to reflect the overwriting report", got.Reason)
	}
}

func TestHealthManager_DoesNotConflateDistinctModels(t *testing.T) {
	h := model.NewHealthManager()
	if err := h.Report("gpt-5.1", model.Health{Status: model.StatusAvailable}); err != nil {
		t.Fatalf("Report failed: %v", err)
	}

	got := h.Get("gemini-3.5-flash")
	if got.Status != model.StatusUnknown {
		t.Errorf("Get on a never-reported model returned %q, want %q (unaffected by a different model's report)", got.Status, model.StatusUnknown)
	}
}

func TestHealth_Unavailable_AvailableIsNeverUnavailable(t *testing.T) {
	h := model.Health{Status: model.StatusAvailable}
	if h.Unavailable() {
		t.Error("Unavailable() = true for StatusAvailable, want false")
	}
}

func TestHealth_Unavailable_UnknownIsNeverUnavailable(t *testing.T) {
	h := model.Health{Status: model.StatusUnknown}
	if h.Unavailable() {
		t.Error("Unavailable() = true for StatusUnknown, want false")
	}
}

func TestHealth_Unavailable_UnavailableWithZeroRetryAtStaysUnavailable(t *testing.T) {
	h := model.Health{Status: model.StatusUnavailable}
	if !h.Unavailable() {
		t.Error("Unavailable() = false for StatusUnavailable with no RetryAt, want true (no known expiry, per Health's own doc comment)")
	}
}

func TestHealth_Unavailable_CooldownWithZeroRetryAtStaysUnavailable(t *testing.T) {
	h := model.Health{Status: model.StatusCooldown}
	if !h.Unavailable() {
		t.Error("Unavailable() = false for StatusCooldown with no RetryAt, want true")
	}
}

func TestHealth_Unavailable_RetryAtInFutureStaysUnavailable(t *testing.T) {
	h := model.Health{Status: model.StatusCooldown, RetryAt: time.Now().Add(time.Hour)}
	if !h.Unavailable() {
		t.Error("Unavailable() = false for a RetryAt still in the future, want true")
	}
}

func TestHealth_Unavailable_RetryAtInPastLiftsAutomatically(t *testing.T) {
	h := model.Health{Status: model.StatusCooldown, RetryAt: time.Now().Add(-time.Hour)}
	if h.Unavailable() {
		t.Error("Unavailable() = true for a RetryAt already in the past, want false (cooldown lifted without a fresh Report)")
	}
}

func TestHealth_Unavailable_UnavailableStatusRetryAtInPastLiftsAutomatically(t *testing.T) {
	h := model.Health{Status: model.StatusUnavailable, RetryAt: time.Now().Add(-time.Minute)}
	if h.Unavailable() {
		t.Error("Unavailable() = true for StatusUnavailable with a past RetryAt, want false")
	}
}

// TestHealthManager_ConcurrentReportAndGet_NoRace proves HealthManager is
// safe under Foundry's own established concurrent-Acts pattern (worktree
// isolation) — more than one Executor could plausibly report health at
// the same time. Run with -race; the test's own assertions are secondary
// to the race detector finding nothing.
func TestHealthManager_ConcurrentReportAndGet_NoRace(t *testing.T) {
	h := model.NewHealthManager()
	var wg sync.WaitGroup
	for range 50 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_ = h.Report("gpt-5.1", model.Health{Status: model.StatusAvailable})
		}()
		go func() {
			defer wg.Done()
			_ = h.Get("gpt-5.1")
		}()
	}
	wg.Wait()
}

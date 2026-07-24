package model_test

import (
	"strings"
	"testing"

	"foundry/model"
)

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := model.NewRegistry()
	info := model.Info{ID: "gpt-5.1", Executor: "openai", Provider: "OpenAI", DisplayName: "GPT-5.1"}
	if err := r.Register(info); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	got, err := r.Get("gpt-5.1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got != info {
		t.Errorf("Get(%q) = %+v, want %+v", "gpt-5.1", got, info)
	}
}

// TestRegistry_RegisterAndGet_RoundTripsCapabilitiesLimitsQuality covers
// ADR-0013's third increment (Proposed): Capabilities/Limits/Quality are
// plain data, round-tripping through Register/Get exactly like every
// other Info field — Info stays a comparable struct (no slice/map fields)
// so this is provable with plain equality, the same as
// TestRegistry_RegisterAndGet above.
func TestRegistry_RegisterAndGet_RoundTripsCapabilitiesLimitsQuality(t *testing.T) {
	r := model.NewRegistry()
	info := model.Info{
		ID: "claude-sonnet-5", Executor: "claude", Provider: "Anthropic", DisplayName: "Claude Sonnet 5",
		Capabilities: model.Capabilities{ToolUse: true, Thinking: true, Streaming: true, Multimodal: true, StructuredOutput: true},
		Limits:       model.Limits{MaxContext: 200000},
		Quality:      model.Quality{Reasoning: 4, Coding: 5, Review: 4},
	}
	if err := r.Register(info); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	got, err := r.Get("claude-sonnet-5")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got != info {
		t.Errorf("Get(%q) = %+v, want %+v", "claude-sonnet-5", got, info)
	}
	if !got.Capabilities.ToolUse || !got.Capabilities.Thinking {
		t.Errorf("Capabilities = %+v, want ToolUse and Thinking both true", got.Capabilities)
	}
	if got.Limits.MaxContext != 200000 {
		t.Errorf("Limits.MaxContext = %d, want 200000", got.Limits.MaxContext)
	}
	if got.Quality.Coding != 5 {
		t.Errorf("Quality.Coding = %d, want 5", got.Quality.Coding)
	}
}

// TestRegistry_ZeroValueCapabilitiesLimitsQualityMeansUnrated covers a
// model registered without any of the new fields (e.g. copilot-default,
// executor/copilotcli.SupportedModels) — the zero value must round-trip
// as "not rated," not fail Register or silently default to something else.
func TestRegistry_ZeroValueCapabilitiesLimitsQualityMeansUnrated(t *testing.T) {
	r := model.NewRegistry()
	info := model.Info{ID: "copilot-default", Executor: "copilot"}
	if err := r.Register(info); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	got, err := r.Get("copilot-default")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.Capabilities != (model.Capabilities{}) {
		t.Errorf("Capabilities = %+v, want the zero value", got.Capabilities)
	}
	if got.Limits != (model.Limits{}) {
		t.Errorf("Limits = %+v, want the zero value", got.Limits)
	}
	if got.Quality != (model.Quality{}) {
		t.Errorf("Quality = %+v, want the zero value", got.Quality)
	}
}

// TestRegistry_Health_NoManagerAttachedReturnsUnknown covers ADR-0013's
// fifth increment (Proposed): a Registry that never had SetHealthManager
// called on it still answers Health queries, always as unknown, rather
// than panicking or erroring.
func TestRegistry_Health_NoManagerAttachedReturnsUnknown(t *testing.T) {
	r := model.NewRegistry()
	got := r.Health("gpt-5.1")
	if got.Status != model.StatusUnknown {
		t.Errorf("Status = %q, want %q", got.Status, model.StatusUnknown)
	}
}

// TestRegistry_Health_DelegatesToAttachedManager proves Registry.Health is
// a pure delegation to whatever HealthManager SetHealthManager attached —
// "Registry queries health manager," not a second, independent health
// store of its own.
func TestRegistry_Health_DelegatesToAttachedManager(t *testing.T) {
	r := model.NewRegistry()
	h := model.NewHealthManager()
	if err := h.Report("gpt-5.1", model.Health{Status: model.StatusUnavailable, Reason: "quota exhausted"}); err != nil {
		t.Fatalf("Report failed: %v", err)
	}
	r.SetHealthManager(h)

	got := r.Health("gpt-5.1")
	if got.Status != model.StatusUnavailable {
		t.Errorf("Status = %q, want %q", got.Status, model.StatusUnavailable)
	}
	if got.Reason != "quota exhausted" {
		t.Errorf("Reason = %q, want %q", got.Reason, "quota exhausted")
	}
}

// TestRegistry_Health_DoesNotRequireModelToBeCatalogued covers a
// deliberate design choice: health tracking is independent of Register —
// querying Health for an ID never registered via Register still delegates
// correctly, rather than requiring catalog membership first.
func TestRegistry_Health_DoesNotRequireModelToBeCatalogued(t *testing.T) {
	r := model.NewRegistry()
	h := model.NewHealthManager()
	if err := h.Report("not-in-catalog", model.Health{Status: model.StatusAvailable}); err != nil {
		t.Fatalf("Report failed: %v", err)
	}
	r.SetHealthManager(h)

	got := r.Health("not-in-catalog")
	if got.Status != model.StatusAvailable {
		t.Errorf("Status = %q, want %q", got.Status, model.StatusAvailable)
	}
}

func TestRegistry_Get_UnregisteredIDFails(t *testing.T) {
	r := model.NewRegistry()
	_, err := r.Get("does-not-exist")
	if err == nil {
		t.Fatal("Get on an unregistered ID returned nil error")
	}
	if !strings.Contains(err.Error(), "does-not-exist") {
		t.Errorf("error = %q, want it to name the missing ID", err.Error())
	}
}

func TestRegistry_Register_RefusesEmptyID(t *testing.T) {
	r := model.NewRegistry()
	err := r.Register(model.Info{Executor: "openai"})
	if err == nil {
		t.Fatal("Register with an empty ID returned nil error")
	}
}

func TestRegistry_Register_RefusesEmptyExecutor(t *testing.T) {
	r := model.NewRegistry()
	err := r.Register(model.Info{ID: "gpt-5.1"})
	if err == nil {
		t.Fatal("Register with an empty Executor returned nil error")
	}
	if !strings.Contains(err.Error(), "gpt-5.1") {
		t.Errorf("error = %q, want it to name the model ID", err.Error())
	}
}

func TestRegistry_Register_RefusesDuplicateID(t *testing.T) {
	r := model.NewRegistry()
	if err := r.Register(model.Info{ID: "gpt-5.1", Executor: "openai"}); err != nil {
		t.Fatalf("first Register failed: %v", err)
	}
	err := r.Register(model.Info{ID: "gpt-5.1", Executor: "openai-compatible"})
	if err == nil {
		t.Fatal("second Register with a duplicate ID (even under a different Executor) returned nil error")
	}
	if !strings.Contains(err.Error(), "gpt-5.1") {
		t.Errorf("error = %q, want it to name the duplicate ID", err.Error())
	}
}

func TestRegistry_List_ReturnsSortedByID(t *testing.T) {
	r := model.NewRegistry()
	for _, id := range []string{"gemini-3.5-flash", "claude-sonnet-5", "gpt-5.1"} {
		if err := r.Register(model.Info{ID: id, Executor: "x"}); err != nil {
			t.Fatalf("Register(%q) failed: %v", id, err)
		}
	}

	got := r.List()
	if len(got) != 3 {
		t.Fatalf("List() returned %d entries, want 3", len(got))
	}
	want := []string{"claude-sonnet-5", "gemini-3.5-flash", "gpt-5.1"}
	for i, id := range want {
		if got[i].ID != id {
			t.Errorf("List()[%d].ID = %q, want %q", i, got[i].ID, id)
		}
	}
}

func TestRegistry_List_EmptyRegistryReturnsEmptySlice(t *testing.T) {
	r := model.NewRegistry()
	got := r.List()
	if len(got) != 0 {
		t.Errorf("List() on an empty Registry = %+v, want empty", got)
	}
}

func TestRegistry_ByExecutor_FiltersAndSorts(t *testing.T) {
	r := model.NewRegistry()
	entries := []model.Info{
		{ID: "gpt-5.1-mini", Executor: "openai"},
		{ID: "gpt-5.1", Executor: "openai"},
		{ID: "gemini-3.5-flash", Executor: "gemini"},
	}
	for _, info := range entries {
		if err := r.Register(info); err != nil {
			t.Fatalf("Register(%q) failed: %v", info.ID, err)
		}
	}

	got := r.ByExecutor("openai")
	if len(got) != 2 {
		t.Fatalf("ByExecutor(%q) returned %d entries, want 2", "openai", len(got))
	}
	if got[0].ID != "gpt-5.1" || got[1].ID != "gpt-5.1-mini" {
		t.Errorf("ByExecutor(%q) = %+v, want sorted [gpt-5.1, gpt-5.1-mini]", "openai", got)
	}
}

func TestRegistry_ByExecutor_UnknownExecutorReturnsEmpty(t *testing.T) {
	r := model.NewRegistry()
	if err := r.Register(model.Info{ID: "gpt-5.1", Executor: "openai"}); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	got := r.ByExecutor("does-not-exist")
	if len(got) != 0 {
		t.Errorf("ByExecutor on an unknown executor = %+v, want empty", got)
	}
}

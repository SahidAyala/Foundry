package project

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/SahidAyala/Foundry/engine"
)

// ExecutorsFile is the conventional location, relative to a project root,
// where LoadExecutorConfig reads a project's Executor configuration
// (RFC-0004 §2, docs/04-guides/multi-executor-router-implementation-plan.md
// Piece 1, Commit 6).
const ExecutorsFile = ".foundry/executors.json"

// ExecutorConfig is one named Executor's project-local configuration: which
// vendor to construct, which model to ask it for, and which environment
// variable holds its API key. LoadExecutorConfig only decodes this shape —
// constructing a real vendor Executor from it is Piece 3 of
// docs/04-guides/multi-executor-router-implementation-plan.md, gated on the
// Executor-contract ADR that plan's Piece 2 proposes writing first.
type ExecutorConfig struct {
	Vendor    string `json:"vendor"`
	Model     string `json:"model"`
	APIKeyEnv string `json:"api_key_env"`

	// BaseURL names the endpoint an "openai-compatible" vendor's Executor
	// calls instead of OpenAI's own — required for that vendor (Ollama,
	// Groq, DeepSeek, and others document an explicit OpenAI-compatible
	// endpoint); ignored by every other vendor.
	BaseURL string `json:"base_url"`

	// TimeoutSeconds overrides how long a single Execute call may run
	// before the Executor gives up, in seconds. Zero (the default) means
	// "use the vendor's own built-in default" — 30 minutes for every
	// CLI-backed vendor ("claude", "gemini", "copilot"), whose deadline
	// has to cover a whole agent session reading the repository with its
	// own tools, and 5 minutes for the HTTP-API vendors ("openai",
	// "gemini-api", "openai-compatible"), which send one completion
	// request.
	//
	// Honored by every vendor whose Executor exposes SetTimeout — all
	// three CLI-backed ones today (cmd/foundry/main.go's withTimeout). An
	// HTTP-API vendor's entry still ignores it: those have no configurable
	// deadline to apply it to.
	TimeoutSeconds int `json:"timeout_seconds"`
}

// LoadExecutorConfig reads and decodes root's conventional Executor
// configuration file (ExecutorsFile) into a map of name to ExecutorConfig,
// the same names a PipelineDocument's Step.Executor pin refers to. A
// missing file is not an error — it decodes to an empty map, mirroring
// FilesystemPipelineSource's "missing directory → no Pipelines": a
// project that never opts in to a project-local Executor sees only the
// process default Executor, exactly as it did before this file's format
// existed.
func LoadExecutorConfig(root string) (map[string]ExecutorConfig, error) {
	path := filepath.Join(root, ExecutorsFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]ExecutorConfig{}, nil
		}
		return nil, fmt.Errorf("project: read executor config %q: %w", path, err)
	}

	var config map[string]ExecutorConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("project: decode executor config %q: %w", path, err)
	}
	return config, nil
}

// ExecutorConstructor constructs a real vendor Executor from its decoded
// ExecutorConfig and the project's workspace directory — the vendor-dispatch
// seam a composition root supplies (ADR-0005 Decision 5,
// docs/03-adrs/ADR-0005-executor-contract-and-capability-model.md),
// mirroring how a plain func(workspace string) engine.Executor already
// supplies the single default Executor. workspace exists so a
// subprocess-based named vendor (e.g. executor/geminicli, which must run in
// a specific directory the same way executor/claude does) can be
// constructed correctly — a pure HTTP-API vendor (executor/openai,
// executor/gemini) simply ignores it, exactly as executor/openai's own
// NewExecutor already takes no workspace parameter at all. Only
// cmd/foundry/main.go — Foundry's true composition root — knows which
// concrete vendor packages exist; project stays vendor-agnostic and only
// calls whatever ExecutorConstructor it is handed.
type ExecutorConstructor func(cfg ExecutorConfig, workspace string) (engine.Executor, error)

// WrapConstructor returns construct with wrap applied to every Executor it
// successfully builds — the seam a composition root needs to attach
// something to the project-configured Executors it never sees again
// afterwards (BuildExecutorRegistry hands back a registry, not its
// contents). Its one use today is attaching live-output narration
// (cli.TraceExecutor); wrap takes and returns nothing, so it can only
// configure an Executor, never substitute or wrap the Executor itself.
//
// A nil construct — every caller that configures no named Executor,
// production and test alike — is returned untouched, so wrapping is safe to
// apply unconditionally.
func WrapConstructor(construct ExecutorConstructor, wrap func(engine.Executor)) ExecutorConstructor {
	if construct == nil {
		return nil
	}
	return func(cfg ExecutorConfig, workspace string) (engine.Executor, error) {
		e, err := construct(cfg, workspace)
		if err != nil {
			return nil, err
		}
		wrap(e)
		return e, nil
	}
}

// BuildExecutorRegistry reads root's ExecutorsFile (via LoadExecutorConfig)
// and constructs an engine.ExecutorRegistry from it, calling construct once
// per named entry. A missing or empty file returns an empty registry
// regardless of construct — a project that never opts in to a project-local
// Executor sees no change, exactly as LoadExecutorConfig's own doc comment
// promises. construct may be nil, meaning "this caller supports
// constructing no named vendor Executors"; a *non-empty* executors.json in
// that case is a clear, named configuration error rather than a silently
// ignored one.
func BuildExecutorRegistry(root string, construct ExecutorConstructor) (*engine.ExecutorRegistry, error) {
	config, err := LoadExecutorConfig(root)
	if err != nil {
		return nil, err
	}
	registry := engine.NewExecutorRegistry()
	if len(config) == 0 {
		return registry, nil
	}
	if construct == nil {
		return nil, fmt.Errorf("project: %s declares %d executor(s), but this caller supports constructing none", ExecutorsFile, len(config))
	}
	for name, cfg := range config {
		exec, err := construct(cfg, root)
		if err != nil {
			return nil, fmt.Errorf("project: build executor %q: %w", name, err)
		}
		if err := registry.Register(name, exec); err != nil {
			return nil, err
		}
	}
	return registry, nil
}

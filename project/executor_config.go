package project

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"foundry/engine"
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
// ExecutorConfig — the vendor-dispatch seam a composition root supplies
// (ADR-0005 Decision 5,
// docs/03-adrs/ADR-0005-executor-contract-and-capability-model.md),
// mirroring how a plain func(workspace string) engine.Executor already
// supplies the single default Executor. Only cmd/foundry/main.go —
// Foundry's true composition root — knows which concrete vendor packages
// (executor/claude, executor/openai, ...) exist; project stays
// vendor-agnostic and only calls whatever ExecutorConstructor it is
// handed.
type ExecutorConstructor func(cfg ExecutorConfig) (engine.Executor, error)

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
		exec, err := construct(cfg)
		if err != nil {
			return nil, fmt.Errorf("project: build executor %q: %w", name, err)
		}
		if err := registry.Register(name, exec); err != nil {
			return nil, err
		}
	}
	return registry, nil
}

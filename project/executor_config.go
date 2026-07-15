package project

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
// FilesystemPipelineProvider's "missing directory → no Pipelines": a
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

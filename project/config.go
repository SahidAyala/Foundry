package project

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ConfigFile is the conventional location, relative to a project root,
// where LoadConfig reads a project's own settings (RFC-0004 §2.6, Piece 4;
// ADR-0010, Piece 6 — both of
// docs/04-guides/multi-executor-router-implementation-plan.md).
const ConfigFile = ".foundry/config.json"

// Config is a project's own settings, read once at composition-root wiring
// time.
type Config struct {
	// DocsPath is the project-relative file a "project-doc" apply Target
	// (RFC-0004 §2.6) writes a Knowledge-lite capture Step's Produced
	// prose into; empty means the project has not opted into that apply
	// target.
	DocsPath string `json:"docs_path"`

	// RequireApprovalBeforeRemotePublish gates PipelineRegistry's
	// SetPublishPolicy (ADR-0010,
	// docs/03-adrs/ADR-0010-vcs-pr-integration-and-apply-targets.md
	// Decision 3): true refuses, at Pipeline-registration time, any
	// Pipeline declaring a "remote-pr" apply Step with no preceding
	// approve Step. false (or the field's absence) means the project has
	// explicitly opted out of requiring one.
	RequireApprovalBeforeRemotePublish bool `json:"require_approval_before_remote_publish"`

	// RemotePublishTokenEnv names the environment variable
	// vcs.GitHubPRApplier reads its GitHub credential from at Apply time
	// (ADR-0010 Decision 7) — never persisted, logged, or passed through
	// domain.Intent or any recorded Evidence, mirroring
	// ExecutorConfig.APIKeyEnv's pattern. Empty means the project has not
	// configured a "remote-pr" apply target at all.
	RemotePublishTokenEnv string `json:"remote_publish_token_env"`
}

// LoadConfig reads and decodes root's conventional configuration file
// (ConfigFile). A missing file is not an error — it decodes to the zero
// Config, mirroring LoadExecutorConfig's "missing file → empty" pattern: a
// project that never opts in sees every field's default (no DocsPath,
// today's only field), exactly as if this file's format didn't exist.
func LoadConfig(root string) (Config, error) {
	path := filepath.Join(root, ConfigFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Config{}, nil
		}
		return Config{}, fmt.Errorf("project: read config %q: %w", path, err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return Config{}, fmt.Errorf("project: decode config %q: %w", path, err)
	}
	return config, nil
}

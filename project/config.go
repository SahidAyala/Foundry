package project

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ConfigFile is the conventional location, relative to a project root,
// where LoadConfig reads a project's own settings (RFC-0004 §2.6, Piece 4
// of docs/04-guides/multi-executor-router-implementation-plan.md). §2.5's
// require_approval_before_remote_publish is a field Piece 6 adds to this
// same file later, gated on its own ADR — LoadConfig's shape does not need
// to change to accommodate that.
const ConfigFile = ".foundry/config.json"

// Config is a project's own settings, read once at composition-root wiring
// time. DocsPath is the project-relative file a "project-doc" apply Target
// (RFC-0004 §2.6) writes a Knowledge-lite capture Step's Produced prose
// into; empty means the project has not opted into that apply target.
type Config struct {
	DocsPath string `json:"docs_path"`
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

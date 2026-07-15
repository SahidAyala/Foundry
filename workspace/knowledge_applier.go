package workspace

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"foundry/domain"
	"foundry/engine"
)

// KnowledgeNoteDir is the conventional, fixed location, relative to a
// project's workspace root, where KnowledgeNoteApplier writes each Act's
// note (RFC-0004 §2.6, Piece 4 of
// docs/04-guides/multi-executor-router-implementation-plan.md). Unlike
// executors.json or config.json, this is not itself configurable —
// Knowledge-lite capture is deliberately a write with no indexing,
// retrieval, or provenance schema (RFC-0004 §2.6 explicitly declines to
// design M4), so there is nothing here for a project to configure yet.
const KnowledgeNoteDir = ".foundry/knowledge"

// KnowledgeNoteApplier implements engine.Applier for a Pipeline's apply
// Step declaring Target: engine.ApplyTargetKnowledgeNote — one of RFC-0004
// §2.6's two Knowledge-lite capture targets. It writes act.Patch — the
// prose a preceding Generate Step produced; Outcome's one content field,
// reused here for prose rather than a code patch, exactly as the "local"
// target already reuses it for a git diff — to a new file under
// KnowledgeNoteDir, named for the Act and a short slug of its Intent so the
// directory stays human-scannable without any index. It never reads or
// aggregates prior notes: this is a write, not a memory system.
type KnowledgeNoteApplier struct{}

// Apply writes act.Patch to KnowledgeNoteDir/<act-id>-<slug>.md under
// workspaceRoot, creating the directory if it does not already exist.
func (KnowledgeNoteApplier) Apply(ctx context.Context, workspaceRoot string, act *domain.Act) error {
	dir := filepath.Join(workspaceRoot, KnowledgeNoteDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("workspace: knowledge-note: create %s: %w", dir, err)
	}
	path := filepath.Join(dir, act.ID+"-"+slugify(act.Intent)+".md")
	content := fmt.Sprintf("# %s\n\nAct: %s\n\n%s\n", act.Intent, act.ID, act.Patch)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("workspace: knowledge-note: write %s: %w", path, err)
	}
	return nil
}

var _ engine.Applier = KnowledgeNoteApplier{}

// ProjectDocApplier implements engine.Applier for a Pipeline's apply Step
// declaring Target: engine.ApplyTargetProjectDoc — RFC-0004 §2.6's other
// Knowledge-lite capture target. DocsPath is the project-relative file it
// appends act.Patch's prose to, resolved from .foundry/config.json's
// docs_path by the composition root (project.LoadConfig) — ProjectDocApplier
// itself knows nothing about that file's format, only where to write.
//
// Apply appends rather than overwrites: unlike a knowledge note (one new
// file per Act), project-doc names one file multiple Acts write into over
// the project's life, so each Act's entry must not erase the ones before
// it.
type ProjectDocApplier struct {
	DocsPath string
}

// Apply appends act.Patch, under a heading naming the Act, to
// a.DocsPath under workspaceRoot, creating the file and its parent
// directory if neither already exists. It returns a clear, named error if
// a.DocsPath is empty — a Pipeline that declares this target without a
// project having configured docs_path is a configuration error, not a
// silent no-op.
func (a ProjectDocApplier) Apply(ctx context.Context, workspaceRoot string, act *domain.Act) error {
	if a.DocsPath == "" {
		return fmt.Errorf("workspace: project-doc: no docs_path configured in .foundry/config.json")
	}
	path := filepath.Join(workspaceRoot, a.DocsPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("workspace: project-doc: create %s: %w", filepath.Dir(path), err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("workspace: project-doc: open %s: %w", path, err)
	}
	defer f.Close()
	entry := fmt.Sprintf("\n## %s (%s)\n\n%s\n", act.Intent, act.ID, act.Patch)
	if _, err := f.WriteString(entry); err != nil {
		return fmt.Errorf("workspace: project-doc: write %s: %w", path, err)
	}
	return nil
}

var _ engine.Applier = ProjectDocApplier{}

// nonAlnum matches any run of characters slugify treats as a word
// separator.
var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// slugify renders s as a short, filesystem-safe fragment: lowercase,
// non-alphanumeric runs collapsed to a single hyphen, leading/trailing
// hyphens trimmed, and capped at 40 characters so a long Intent can't
// produce an unwieldy filename. An Intent with no alphanumeric characters
// at all slugifies to "note" rather than an empty string.
func slugify(s string) string {
	slug := nonAlnum.ReplaceAllString(strings.ToLower(s), "-")
	slug = strings.Trim(slug, "-")
	if len(slug) > 40 {
		slug = strings.Trim(slug[:40], "-")
	}
	if slug == "" {
		slug = "note"
	}
	return slug
}

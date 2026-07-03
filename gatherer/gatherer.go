// Package gatherer assembles the considered Context for an Act. The M0
// implementation is naive by design: it extracts file names mentioned in the
// Intent's text and returns their contents from the repository, bounded in
// total size. Semantic retrieval and Knowledge-based context are deferred
// (docs/04-guides/M0-IMPLEMENTATION-BACKLOG.md, M0.1).
package gatherer

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

// maxContextBytes bounds the total gathered content per Act so an Intent
// naming huge files cannot produce an unbounded Executor prompt.
const maxContextBytes = 100 * 1024

// tokenPattern matches candidate path tokens in an Intent: runs of
// path-like characters. Filtering to tokens that look like file names
// happens in extractFileNames.
var tokenPattern = regexp.MustCompile(`[A-Za-z0-9_\-./]+`)

// NaiveGatherer satisfies the engine.Gatherer port by reading files named in
// the Intent from a fixed repository root.
type NaiveGatherer struct {
	repoPath string
}

// NewNaiveGatherer returns a Gatherer that reads context from repoPath.
func NewNaiveGatherer(repoPath string) *NaiveGatherer {
	return &NaiveGatherer{repoPath: repoPath}
}

var _ engine.Gatherer = (*NaiveGatherer)(nil)

// Gather extracts file names from the Intent's text and returns one entry
// per name, in order of first mention: the file's contents when it exists,
// or a "not found" / "refused" marker otherwise. Missing files are never a
// hard error; the Executor sees what was and wasn't available. Total output
// is bounded by maxContextBytes.
func (g *NaiveGatherer) Gather(ctx context.Context, intent *domain.Intent) ([]string, error) {
	var (
		gathered  []string
		remaining = maxContextBytes
	)

	for _, name := range extractFileNames(intent.Text) {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("gatherer: %w", err)
		}

		full, ok := g.resolve(name)
		if !ok {
			gathered = append(gathered, name+": refused (escapes repository)")
			continue
		}

		content, err := os.ReadFile(full)
		if err != nil {
			gathered = append(gathered, name+": not found")
			continue
		}

		entry := name + ":\n" + string(content)
		if len(entry) > remaining {
			entry = entry[:remaining] + "\n[truncated]"
			remaining = 0
		} else {
			remaining -= len(entry)
		}
		gathered = append(gathered, entry)

		if remaining == 0 {
			break
		}
	}

	return gathered, nil
}

// resolve joins name to the repository root and reports whether the result
// stays inside it, refusing absolute paths and traversal.
func (g *NaiveGatherer) resolve(name string) (string, bool) {
	if filepath.IsAbs(name) {
		return "", false
	}
	full := filepath.Join(g.repoPath, filepath.Clean(name))
	rel, err := filepath.Rel(g.repoPath, full)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return full, true
}

// extractFileNames returns the file-name-like tokens in text — tokens whose
// last dot separates a non-empty base from a non-empty extension — deduped,
// in order of first mention. Trailing dots (sentence punctuation) are
// trimmed first.
func extractFileNames(text string) []string {
	var (
		names []string
		seen  = map[string]bool{}
	)
	for _, token := range tokenPattern.FindAllString(text, -1) {
		token = strings.TrimRight(token, ".")
		dot := strings.LastIndex(token, ".")
		if dot <= 0 || dot == len(token)-1 {
			continue
		}
		if strings.HasSuffix(token[:dot], "/") {
			continue
		}
		if seen[token] {
			continue
		}
		seen[token] = true
		names = append(names, token)
	}
	return names
}

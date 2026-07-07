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
	"sort"
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

// Gather returns considered context in two phases, both bounded by one
// shared maxContextBytes budget. First, the files named in the Intent's
// text, in order of first mention: contents when the file exists, or a
// "not found" / "refused" marker otherwise — missing files are never a hard
// error. Second, supplementary context: the repository's README.md and the
// files sharing a directory with the named files that were found, ordered
// by priority (config files, then docs, then code) so that when the budget
// truncates, the highest-priority context survives.
func (g *NaiveGatherer) Gather(ctx context.Context, intent *domain.Intent) ([]string, error) {
	var (
		gathered  []string
		remaining = maxContextBytes
		included  = make(map[string]bool)
		dirs      []string
		seenDir   = make(map[string]bool)
	)

	// appendBounded adds one file entry within the remaining budget,
	// truncating the entry that exhausts it. It reports whether any budget
	// remains.
	appendBounded := func(entry string) bool {
		if len(entry) > remaining {
			entry = entry[:remaining] + "\n[truncated]"
			remaining = 0
		} else {
			remaining -= len(entry)
		}
		gathered = append(gathered, entry)
		return remaining > 0
	}

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

		included[name] = true
		if dir := filepath.Dir(name); !seenDir[dir] {
			seenDir[dir] = true
			dirs = append(dirs, dir)
		}
		if !appendBounded(name + ":\n" + string(content)) {
			return gathered, nil
		}
	}

	for _, name := range g.supplementary(dirs, included) {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("gatherer: %w", err)
		}

		content, err := os.ReadFile(filepath.Join(g.repoPath, name))
		if err != nil {
			continue
		}
		if !appendBounded(name + ":\n" + string(content)) {
			return gathered, nil
		}
	}

	return gathered, nil
}

// supplementary returns the not-yet-included context candidates — the
// repository's README.md and the regular files in dirs — sorted by
// contextPriority, then lexicographically. Candidate paths are
// repository-relative; dirs come from named files, so they cannot escape.
func (g *NaiveGatherer) supplementary(dirs []string, included map[string]bool) []string {
	var names []string

	add := func(name string) {
		if included[name] {
			return
		}
		included[name] = true
		names = append(names, name)
	}

	if _, err := os.Stat(filepath.Join(g.repoPath, "README.md")); err == nil {
		add("README.md")
	}

	for _, dir := range dirs {
		entries, err := os.ReadDir(filepath.Join(g.repoPath, dir))
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.Type().IsRegular() {
				continue
			}
			name := entry.Name()
			if dir != "." {
				name = dir + "/" + name
			}
			add(name)
		}
	}

	sort.SliceStable(names, func(i, j int) bool {
		pi, pj := contextPriority(names[i]), contextPriority(names[j])
		if pi != pj {
			return pi < pj
		}
		return names[i] < names[j]
	})
	return names
}

// contextPriority ranks supplementary context: config files first (0), docs
// second (1), code last (2), so budget truncation drops code before docs
// and docs before config.
func contextPriority(name string) int {
	switch filepath.Base(name) {
	case "go.mod", "go.sum", "Makefile", ".gitignore":
		return 0
	}
	switch filepath.Ext(name) {
	case ".json", ".yaml", ".yml", ".toml":
		return 0
	case ".md", ".txt":
		return 1
	}
	return 2
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

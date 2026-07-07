// Package gatherer assembles the considered Context for an Act. The M0
// implementation is naive by design: it extracts file names mentioned in the
// Intent's text and returns their contents from the repository, bounded in
// total size. Semantic retrieval and Knowledge-based context are deferred
// (docs/04-guides/M0-IMPLEMENTATION-BACKLOG.md, M0.1).
package gatherer

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
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

// identifierPattern matches capitalized-word-like tokens (e.g. "User",
// "Account") that look like a type or function name, for the identifier
// fallback in Gather: an Intent that names an entity rather than a file
// (e.g. "rename User to Account") still needs to find relevant files.
var identifierPattern = regexp.MustCompile(`\b[A-Z][A-Za-z0-9]{2,}\b`)

const (
	// maxIdentifierMatches bounds how many files the identifier fallback
	// contributes, so a common word cannot flood the gathered context.
	maxIdentifierMatches = 5
	// maxIdentifierScanFiles bounds how much of the repository the
	// fallback's content scan reads, so it stays cheap on a large repo.
	maxIdentifierScanFiles = 2000
)

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

// Gather returns considered context in up to three phases, all bounded by
// one shared maxContextBytes budget. First, the files named in the Intent's
// text, in order of first mention: contents when the file exists, or a
// "not found" / "refused" marker otherwise — missing files are never a hard
// error. Second, if no named file resolved — an Intent like "rename User to
// Account" names an entity, not a file — the identifier fallback: files
// whose content mentions a capitalized identifier from the Intent, bounded
// and deterministically ordered. Third, supplementary context: the
// repository's README.md and the files sharing a directory with whatever
// resolved above, ordered by priority (config files, then docs, then code)
// so that when the budget truncates, the highest-priority context survives.
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

	// include records name as resolved (for supplementary's directory scan
	// and dedup) and appends its content within budget.
	include := func(name, content string) bool {
		included[name] = true
		if dir := filepath.Dir(name); !seenDir[dir] {
			seenDir[dir] = true
			dirs = append(dirs, dir)
		}
		return appendBounded(name + ":\n" + content)
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

		if !include(name, string(content)) {
			return gathered, nil
		}
	}

	if len(included) == 0 {
		for _, name := range g.findByIdentifier(extractIdentifiers(intent.Text)) {
			if err := ctx.Err(); err != nil {
				return nil, fmt.Errorf("gatherer: %w", err)
			}
			content, err := os.ReadFile(filepath.Join(g.repoPath, name))
			if err != nil {
				continue
			}
			if !include(name, string(content)) {
				return gathered, nil
			}
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

// extractIdentifiers returns the capitalized-word-like tokens in text,
// deduped, in order of first mention.
func extractIdentifiers(text string) []string {
	var (
		ids  []string
		seen = map[string]bool{}
	)
	for _, tok := range identifierPattern.FindAllString(text, -1) {
		if seen[tok] {
			continue
		}
		seen[tok] = true
		ids = append(ids, tok)
	}
	return ids
}

// findByIdentifier walks the repository for regular, non-hidden files whose
// content contains any of ids, returning repository-relative paths in a
// deterministic (sorted) order, capped at maxIdentifierMatches and reading
// at most maxIdentifierScanFiles files. It is a last-resort fallback for an
// Intent that names an entity rather than a file, so a scan error midway
// (permissions, an unreadable file) is not fatal: whatever matched so far is
// still useful context.
func (g *NaiveGatherer) findByIdentifier(ids []string) []string {
	if len(ids) == 0 {
		return nil
	}

	var matches []string
	scanned := 0
	filepath.WalkDir(g.repoPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if len(matches) >= maxIdentifierMatches || scanned >= maxIdentifierScanFiles {
			return filepath.SkipAll
		}
		if d.IsDir() {
			if path != g.repoPath && (strings.HasPrefix(d.Name(), ".") || d.Name() == "vendor" || d.Name() == "node_modules") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			return nil
		}
		scanned++

		content, err := os.ReadFile(path)
		if err != nil || len(content) > maxContextBytes {
			return nil
		}
		for _, id := range ids {
			if bytes.Contains(content, []byte(id)) {
				if rel, err := filepath.Rel(g.repoPath, path); err == nil {
					matches = append(matches, filepath.ToSlash(rel))
				}
				break
			}
		}
		return nil
	})

	sort.Strings(matches)
	return matches
}

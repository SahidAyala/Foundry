// Package knowledge implements Foundry's first read side over Authored
// Knowledge (RFC-0005, docs/01-rfcs/RFC-0005-authored-knowledge-retrieval.md):
// a Context Source (docs/02-architecture/extensibility.md) that retrieves
// notes previously written under workspace.KnowledgeNoteDir
// (RFC-0004 §2.6's Knowledge-lite capture, "knowledge-note" apply target)
// back into a later Act's considered Evidence.
//
// Gather is deliberately naive — lexical word-overlap matching, no
// embeddings, no index — mirroring gatherer.NaiveGatherer's own posture
// for repository files. It is not the whole of M4
// (docs/00-overview/roadmap.md): no note schema, no Derived Knowledge
// index, and no semantic retrieval exist here; RFC-0005 §3 names all
// three as deliberately deferred, not decided.
package knowledge

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
	"foundry/workspace"
)

const (
	// maxNotes bounds how many notes Gather returns, so a large Knowledge
	// corpus cannot flood an Executor's Context with marginal matches.
	maxNotes = 3

	// maxContextBytes bounds Gather's total returned content — a budget
	// separate from gatherer.NaiveGatherer's own, since the two Gatherers
	// compose (gatherer.Compose) rather than share one budget.
	maxContextBytes = 20 * 1024

	// minWordLength excludes short, low-signal tokens ("to", "add", "the")
	// from the lexical match; combined with commonWords, it keeps
	// overlapScore meaningful without a real stopword list.
	minWordLength = 4
)

// wordPattern matches word-like tokens for the naive lexical match: runs of
// letters, digits, and apostrophes starting with a letter.
var wordPattern = regexp.MustCompile(`[A-Za-z][A-Za-z0-9']*`)

// commonWords are short, high-frequency English words that would otherwise
// pollute the lexical match despite meeting minWordLength — a small,
// pragmatic list, not an exhaustive stopword dictionary. Naive is the
// deliberate posture (RFC-0005 §2.3); this list only keeps it from being
// naive to the point of uselessness.
var commonWords = map[string]bool{
	"this": true, "that": true, "with": true, "from": true,
	"have": true, "will": true, "your": true, "into": true,
	"them": true, "then": true, "than": true, "when": true,
	"what": true, "just": true, "some": true, "more": true,
	"also": true, "each": true, "were": true, "been": true,
	"does": true, "make": true, "like": true, "over": true,
	"only": true, "such": true, "very": true, "even": true,
}

// Gatherer implements engine.Gatherer by retrieving Authored Knowledge
// notes (workspace.KnowledgeNoteDir) relevant to an Intent.
type Gatherer struct {
	repoPath string
}

// NewGatherer returns a Gatherer that reads Knowledge notes from repoPath's
// conventional directory.
func NewGatherer(repoPath string) *Gatherer {
	return &Gatherer{repoPath: repoPath}
}

var _ engine.Gatherer = (*Gatherer)(nil)

// note is one candidate Knowledge note, scored against an Intent's
// significant words.
type note struct {
	name    string
	content string
	score   int
}

// Gather returns the considered Context Authored Knowledge contributes to
// intent: up to maxNotes notes from workspace.KnowledgeNoteDir whose
// content shares the most significant words with intent.Text, each
// attributed to its source file (RFC-0005 §2.3's provenance requirement),
// ranked by overlap score (ties broken by filename for determinism), and
// bounded by maxContextBytes. A missing Knowledge directory is not an
// error — nil, exactly as if this Gatherer were never composed in
// (gatherer.Compose) — and an Intent with no significant words matches
// nothing.
func (g *Gatherer) Gather(ctx context.Context, intent *domain.Intent) ([]string, error) {
	dir := filepath.Join(g.repoPath, workspace.KnowledgeNoteDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("knowledge: read %s: %w", dir, err)
	}

	terms := significantWords(intent.Text)
	if len(terms) == 0 {
		return nil, nil
	}

	var candidates []note
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("knowledge: %w", err)
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		content, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		if score := overlapScore(terms, string(content)); score > 0 {
			candidates = append(candidates, note{name: entry.Name(), content: string(content), score: score})
		}
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		return candidates[i].name < candidates[j].name
	})
	if len(candidates) > maxNotes {
		candidates = candidates[:maxNotes]
	}

	var gathered []string
	remaining := maxContextBytes
	for _, c := range candidates {
		entry := filepath.ToSlash(filepath.Join(workspace.KnowledgeNoteDir, c.name)) + ":\n" + c.content
		if len(entry) > remaining {
			gathered = append(gathered, entry[:remaining]+"\n[truncated]")
			break
		}
		remaining -= len(entry)
		gathered = append(gathered, entry)
	}
	return gathered, nil
}

// significantWords returns text's lowercase word-like tokens, excluding
// short and common ones, deduped, in order of first appearance.
func significantWords(text string) []string {
	var words []string
	seen := make(map[string]bool)
	for _, tok := range wordPattern.FindAllString(text, -1) {
		w := strings.ToLower(tok)
		if len(w) < minWordLength || commonWords[w] || seen[w] {
			continue
		}
		seen[w] = true
		words = append(words, w)
	}
	return words
}

// overlapScore counts how many of terms appear in content, case-insensitive.
func overlapScore(terms []string, content string) int {
	lower := strings.ToLower(content)
	score := 0
	for _, t := range terms {
		if strings.Contains(lower, t) {
			score++
		}
	}
	return score
}

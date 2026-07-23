// Package ticket defines the port session's /issue command fetches an
// external ticket's content through — the same "small interface, one
// concrete implementation per vendor" shape engine.Executor and
// engine.Applier already establish, applied to a new kind of external
// system boundary (docs/02-architecture/system-context.md): an issue
// tracker, not a model vendor or a version-control host.
//
// A ticket's content becomes a domain.Intent's text (session.IssueCommand)
// — the same domain concept a human typing "/feature \"...\"" already
// produces, just populated from a different source. This is deliberately
// not a new Step kind (RFC-0002 §4.2's closed five — generate, verify,
// approve, apply, record — stay exactly as they are): fetching a ticket
// happens before a Pipeline ever starts, the same way a slash command's
// own typed argument text already does.
package ticket

import "context"

// Issue is one fetched ticket's content, normalized to the same shape
// regardless of which vendor Fetcher produced it.
type Issue struct {
	// ID is the ticket's own identifier in its vendor's own form — a bare
	// number for GitHub/GitLab issues, a key like "PROJ-123" for Jira.
	ID string

	Title       string
	Description string

	// URL, when the vendor exposes one, lets a later reader of the
	// recorded Act (`foundry show`) trace back to the source ticket —
	// Foundry never needs to record anything beyond the Intent's own text
	// to preserve that link.
	URL string
}

// Fetcher fetches one Issue from an external ticketing system, identified
// by id in whatever form that system's own API expects.
type Fetcher interface {
	Fetch(ctx context.Context, id string) (Issue, error)
}

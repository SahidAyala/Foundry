package session

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"foundry/ticket"
)

// IssueCommand implements /issue <id>: fetches id from the Session's
// configured ticket.Fetcher (SetTicketFetcher), builds a domain.Intent
// from its title and description, and delegates the entire
// generate/verify/approve/apply/record lifecycle to RunPipelineCommand —
// exactly the same flow /feature and /bug already use. The only
// difference from those is where the Intent's text comes from: a fetched
// ticket instead of typed args.
type IssueCommand struct {
	// PipelineName is the Pipeline /issue runs once the Intent is built,
	// mirroring RunPipelineCommand's own field of the same name.
	PipelineName string
}

var _ CommandHandler = IssueCommand{}

// Describe returns this command's one-line /help description.
func (cmd IssueCommand) Describe() string {
	return fmt.Sprintf("Fetch an issue from your configured ticket provider and run the %q Pipeline over it.", cmd.PipelineName)
}

// Run fetches args (an issue id) via s's configured ticket.Fetcher, then
// runs cmd.PipelineName over the fetched Issue's own title and
// description as the Act's Intent text.
func (cmd IssueCommand) Run(ctx context.Context, s *Session, args string) error {
	id := strings.TrimSpace(args)
	if id == "" {
		return errors.New("session: /issue requires an issue id, e.g. /issue 42")
	}
	if s.ticketFetcher == nil {
		return errors.New(`session: /issue: no ticket provider configured — set "ticket_provider" in .foundry/config.json (e.g. "github")`)
	}

	issue, err := s.ticketFetcher.Fetch(ctx, id)
	if err != nil {
		return fmt.Errorf("session: /issue: %w", err)
	}

	return RunPipelineCommand{PipelineName: cmd.PipelineName}.Run(ctx, s, formatIssueIntent(issue))
}

// formatIssueIntent renders a fetched ticket.Issue as Intent text — title
// and description verbatim, plus the issue's own URL (when the vendor
// gives one) so a later reader of the recorded Act (foundry show) can
// trace back to the source ticket. Foundry never needs to record
// anything beyond this plain Intent text to preserve that link.
func formatIssueIntent(issue ticket.Issue) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Implement issue #%s: %s\n\n", issue.ID, issue.Title)
	b.WriteString(issue.Description)
	if issue.URL != "" {
		fmt.Fprintf(&b, "\n\n(Source: %s)", issue.URL)
	}
	return b.String()
}

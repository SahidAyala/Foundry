// Package session implements Foundry's interactive, slash-command-driven
// session — the REPL, its command dispatch, and the handlers that
// translate a slash command into running a Pipeline. It knows nothing
// about how a Pipeline document is authored or discovered (that is
// package project) and nothing about Engine internals beyond the ports
// and types engine already exports (Engine, PipelineRegistry, Pipeline).
package session

import "strings"

// Command is one parsed slash command: its name (without the leading
// "/") and everything typed after it.
type Command struct {
	Name string
	Args string
}

// ParseLine parses one line of session input. A line beginning with "/"
// is a slash command: Name is its first whitespace-delimited token,
// lowercased, with the leading "/" stripped; Args is everything after
// that token, trimmed of surrounding whitespace. Any other line —
// including an empty or whitespace-only one — is not a slash command;
// ParseLine reports isSlash=false and leaves the decision of what plain
// text means (RFC-0002 §8: it becomes an Intent for the default
// Pipeline) to the caller.
func ParseLine(line string) (cmd Command, isSlash bool) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "/") {
		return Command{}, false
	}

	rest := strings.TrimPrefix(line, "/")
	name, args, _ := strings.Cut(rest, " ")
	return Command{
		Name: strings.ToLower(name),
		Args: strings.TrimSpace(args),
	}, true
}

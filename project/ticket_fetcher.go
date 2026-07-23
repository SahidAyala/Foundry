package project

import "foundry/ticket"

// TicketFetcherConstructor constructs a ticket.Fetcher for cfg's
// TicketProvider and the project's workspace directory — the same
// vendor-dispatch seam ExecutorConstructor already establishes for
// Executors (ADR-0005 Decision 5), applied to /issue's own external system
// boundary. Only cmd/foundry/main.go — Foundry's true composition root —
// knows which concrete ticket package (ticket/github, ...) exists;
// project and session stay vendor-agnostic, calling only whatever
// TicketFetcherConstructor they are handed.
type TicketFetcherConstructor func(cfg Config, workspace string) (ticket.Fetcher, error)

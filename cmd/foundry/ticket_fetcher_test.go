package main

import (
	"strings"
	"testing"

	"foundry/project"
	githubticket "foundry/ticket/github"
)

func TestNewTicketFetcher_GithubVendorConstructsGithubFetcher(t *testing.T) {
	fetcher, err := newTicketFetcher(project.Config{TicketProvider: "github"}, "/repo")
	if err != nil {
		t.Fatalf("newTicketFetcher failed: %v", err)
	}
	if _, ok := fetcher.(*githubticket.Fetcher); !ok {
		t.Errorf("newTicketFetcher(github) = %T, want *github.Fetcher", fetcher)
	}
}

func TestNewTicketFetcher_UnsupportedProviderFails(t *testing.T) {
	_, err := newTicketFetcher(project.Config{TicketProvider: "jira"}, "/repo")
	if err == nil {
		t.Fatal("newTicketFetcher with an unsupported provider returned nil error")
	}
	if !strings.Contains(err.Error(), "jira") {
		t.Errorf("error = %q, want it to name the unsupported provider", err)
	}
}

package main

import (
	"strings"
	"testing"

	"foundry/project"
	githubticket "foundry/ticket/github"
	jiraticket "foundry/ticket/jira"
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

func TestNewTicketFetcher_JiraVendorConstructsJiraFetcher(t *testing.T) {
	t.Setenv("FOUNDRY_TEST_JIRA_TOKEN", "test-token-value")

	fetcher, err := newTicketFetcher(project.Config{
		TicketProvider:  "jira",
		JiraBaseURL:     "https://example.atlassian.net",
		JiraEmail:       "test@example.com",
		JiraAPITokenEnv: "FOUNDRY_TEST_JIRA_TOKEN",
	}, "/repo")
	if err != nil {
		t.Fatalf("newTicketFetcher failed: %v", err)
	}
	if _, ok := fetcher.(*jiraticket.Fetcher); !ok {
		t.Errorf("newTicketFetcher(jira) = %T, want *jira.Fetcher", fetcher)
	}
}

func TestNewTicketFetcher_UnsupportedProviderFails(t *testing.T) {
	_, err := newTicketFetcher(project.Config{TicketProvider: "asana"}, "/repo")
	if err == nil {
		t.Fatal("newTicketFetcher with an unsupported provider returned nil error")
	}
	if !strings.Contains(err.Error(), "asana") {
		t.Errorf("error = %q, want it to name the unsupported provider", err)
	}
}

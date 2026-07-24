package main

import (
	"strings"
	"testing"

	"foundry/project"
	asanaticket "foundry/ticket/asana"
	githubticket "foundry/ticket/github"
	gitlabticket "foundry/ticket/gitlab"
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

func TestNewTicketFetcher_GitlabVendorConstructsGitlabFetcher(t *testing.T) {
	fetcher, err := newTicketFetcher(project.Config{TicketProvider: "gitlab"}, "/repo")
	if err != nil {
		t.Fatalf("newTicketFetcher failed: %v", err)
	}
	if _, ok := fetcher.(*gitlabticket.Fetcher); !ok {
		t.Errorf("newTicketFetcher(gitlab) = %T, want *gitlab.Fetcher", fetcher)
	}
}

func TestNewTicketFetcher_AsanaVendorConstructsAsanaFetcher(t *testing.T) {
	t.Setenv("FOUNDRY_TEST_ASANA_TOKEN", "test-token-value")

	fetcher, err := newTicketFetcher(project.Config{
		TicketProvider:   "asana",
		AsanaAPITokenEnv: "FOUNDRY_TEST_ASANA_TOKEN",
	}, "/repo")
	if err != nil {
		t.Fatalf("newTicketFetcher failed: %v", err)
	}
	if _, ok := fetcher.(*asanaticket.Fetcher); !ok {
		t.Errorf("newTicketFetcher(asana) = %T, want *asana.Fetcher", fetcher)
	}
}

func TestNewTicketFetcher_UnsupportedProviderFails(t *testing.T) {
	_, err := newTicketFetcher(project.Config{TicketProvider: "trello"}, "/repo")
	if err == nil {
		t.Fatal("newTicketFetcher with an unsupported provider returned nil error")
	}
	if !strings.Contains(err.Error(), "trello") {
		t.Errorf("error = %q, want it to name the unsupported provider", err)
	}
}

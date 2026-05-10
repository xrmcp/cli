package cmd

import "testing"

func TestSearchRegistryToolsMatchesAcrossFields(t *testing.T) {
	tools := []registryToolRecord{
		{
			Name:        "get_jira_ticket",
			DisplayName: "Get Jira Ticket",
			Description: "Fetch a Jira issue by its key.",
			Category:    "jira",
			Tags:        []string{"atlassian", "ticket"},
			Path:        "jira/get_jira_ticket.xrmcp.json",
		},
		{
			Name:        "list_project",
			DisplayName: "List GitLab Group Projects",
			Description: "List projects in a GitLab group.",
			Category:    "gitlab",
			Tags:        []string{"projects", "git"},
			Path:        "gitlab/list_project.xrmcp.json",
		},
	}

	matches := searchRegistryTools(tools, "atlassian")
	if len(matches) != 1 || matches[0].Name != "get_jira_ticket" {
		t.Fatalf("expected jira tool match by tag, got %#v", matches)
	}

	matches = searchRegistryTools(tools, "gitlab/list_project")
	if len(matches) != 1 || matches[0].Name != "list_project" {
		t.Fatalf("expected gitlab tool match by install id, got %#v", matches)
	}

	matches = searchRegistryTools(tools, "issue")
	if len(matches) != 1 || matches[0].Name != "get_jira_ticket" {
		t.Fatalf("expected jira tool match by description, got %#v", matches)
	}
}

func TestRegistryToolInstallIDFallbacksToPath(t *testing.T) {
	tool := registryToolRecord{
		Name: "custom_tool",
		Path: "custom/custom_tool.xrmcp.json",
	}

	if got := tool.InstallID(); got != "custom/custom_tool" {
		t.Fatalf("expected path-derived install id, got %q", got)
	}
}

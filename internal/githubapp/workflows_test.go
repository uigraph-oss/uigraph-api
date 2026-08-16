package githubapp

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestSeedIsDeterministicAndExact(t *testing.T) {
	first, err := Seed("Acme", "payments-api", "https://github.com/acme/payments-api", "Platform")
	if err != nil {
		t.Fatal(err)
	}
	second, err := Seed("Acme", "payments-api", "https://github.com/acme/payments-api", "Platform")
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatal("seed is not deterministic")
	}
	for _, expected := range []string{"name: Acme", "name: Payments API", "provider: github", "url: https://github.com/acme/payments-api", "team: Platform", "category: service"} {
		if !strings.Contains(string(first), expected) {
			t.Fatalf("seed does not contain %q:\n%s", expected, first)
		}
	}
}

func TestGenerationAndSyncWorkflowBoundaries(t *testing.T) {
	generation := string(GenerateWorkflow("onboarding-id", "trunk"))
	var generationYAML map[string]any
	if err := yaml.Unmarshal([]byte(generation), &generationYAML); err != nil {
		t.Fatalf("generation workflow is invalid YAML: %v", err)
	}
	for _, expected := range []string{
		"@uigraph/agents@experimental artifacts init --seeded",
		"go install github.com/uigraph-oss/uigraph-cli@main",
		"uigraph-cli sync --dry-run",
		"AI_PROVIDER_API_KEY: ${{ secrets.AI_PROVIDER_API_KEY }}",
		"branch: uigraph/generated/onboarding-id",
		"base: trunk",
		".uigraph.yaml",
		".uigraph/**",
	} {
		if !strings.Contains(generation, expected) {
			t.Fatalf("generation workflow does not contain %q", expected)
		}
	}
	if strings.Contains(generation, "UIGRAPH_TOKEN") {
		t.Fatal("generation workflow must not receive UIGRAPH_TOKEN")
	}
	sync := string(SyncWorkflow())
	var syncYAML map[string]any
	if err := yaml.Unmarshal([]byte(sync), &syncYAML); err != nil {
		t.Fatalf("sync workflow is invalid YAML: %v", err)
	}
	if !strings.Contains(sync, "workflow_dispatch:") || !strings.Contains(sync, "push:") || strings.Contains(sync, "pull_request:") {
		t.Fatal("sync workflow must run after artifact pushes and allow manual retry")
	}
	if !strings.Contains(sync, "UIGRAPH_TOKEN: ${{ secrets.UIGRAPH_ONBOARDING_TOKEN }}") {
		t.Fatal("sync workflow does not map the onboarding token")
	}
}

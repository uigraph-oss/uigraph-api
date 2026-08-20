package githubapp

import (
	"maps"
	"slices"
	"strings"
	"testing"
)

func TestWorkflowsSelfHosted(t *testing.T) {
	files, err := Workflows("main", false)
	if err != nil {
		t.Fatalf("Workflows: %v", err)
	}
	want := []string{ArtifactWorkflowPath, SyncWorkflowPath, OnboardingWorkflowPath}
	for _, path := range want {
		if _, ok := files[path]; !ok {
			t.Fatalf("missing %s, got %v", path, slices.Sorted(maps.Keys(files)))
		}
	}
	if len(files) != len(want) {
		t.Fatalf("got %d files, want %d", len(files), len(want))
	}
	sync := string(files[SyncWorkflowPath])
	if !strings.Contains(sync, "UIGRAPH_GATEWAY_URL") {
		t.Fatal("self-hosted sync workflow must still read UIGRAPH_GATEWAY_URL")
	}
	if strings.Contains(sync, "--enterprise") {
		t.Fatal("self-hosted sync workflow must not pass --enterprise")
	}
	if !strings.Contains(sync, `- "main"`) {
		t.Fatalf("default branch was not substituted: %s", sync)
	}
}

func TestWorkflowsEnterprise(t *testing.T) {
	files, err := Workflows("main", true)
	if err != nil {
		t.Fatalf("Workflows: %v", err)
	}
	want := []string{EnterpriseArtifactWorkflowPath, EnterpriseSyncWorkflowPath, EnterpriseOnboardingWorkflowPath}
	for _, path := range want {
		if _, ok := files[path]; !ok {
			t.Fatalf("missing %s, got %v", path, slices.Sorted(maps.Keys(files)))
		}
	}
	if len(files) != len(want) {
		t.Fatalf("got %d files, want %d", len(files), len(want))
	}
	for path, content := range files {
		body := string(content)
		if strings.Contains(body, "UIGRAPH_API_URL") || strings.Contains(body, "UIGRAPH_GATEWAY_URL") {
			t.Fatalf("%s must not require the instance URLs", path)
		}
		if !strings.Contains(body, "--enterprise") {
			t.Fatalf("%s must run its command with --enterprise", path)
		}
	}
	onboarding := string(files[EnterpriseOnboardingWorkflowPath])
	if !strings.Contains(onboarding, "uses: ./"+EnterpriseSyncWorkflowPath) {
		t.Fatalf("onboarding workflow must call the enterprise sync workflow: %s", onboarding)
	}
}

func TestIsOnboardingWorkflowPath(t *testing.T) {
	if !IsOnboardingWorkflowPath(OnboardingWorkflowPath) || !IsOnboardingWorkflowPath(EnterpriseOnboardingWorkflowPath) {
		t.Fatal("both onboarding workflow names must be recognised")
	}
	if IsOnboardingWorkflowPath(SyncWorkflowPath) {
		t.Fatal("the sync workflow is not an onboarding workflow")
	}
}

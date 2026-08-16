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

func TestWorkflowTriggersOnlyOnTheOnboardingBranchAndPullRequests(t *testing.T) {
	workflow := string(Workflow("trunk"))
	var parsed struct {
		On struct {
			Push struct {
				Branches []string `yaml:"branches"`
			} `yaml:"push"`
			PullRequest struct {
				Branches []string `yaml:"branches"`
			} `yaml:"pull_request"`
			WorkflowDispatch any `yaml:"workflow_dispatch"`
		} `yaml:"on"`
	}
	if err := yaml.Unmarshal([]byte(workflow), &parsed); err != nil {
		t.Fatalf("workflow is invalid YAML: %v", err)
	}
	if strings.Join(parsed.On.Push.Branches, ",") != "uigraph/onboarding/**" {
		t.Fatalf("push branches = %v", parsed.On.Push.Branches)
	}
	if strings.Join(parsed.On.PullRequest.Branches, ",") != "trunk" {
		t.Fatalf("pull request branches = %v", parsed.On.PullRequest.Branches)
	}
	if parsed.On.WorkflowDispatch != nil {
		t.Fatal("the workflow file never reaches the default branch, so workflow_dispatch cannot work")
	}
	for _, expected := range []string{
		"@uigraph/agents@experimental artifacts init --seeded",
		"go install github.com/uigraph-oss/uigraph-cli@main",
		"uigraph-cli sync --dry-run",
		"AI_PROVIDER_API_KEY: ${{ secrets.AI_PROVIDER_API_KEY }}",
		"UIGRAPH_TOKEN: ${{ secrets.UIGRAPH_ONBOARDING_TOKEN }}",
		"Generation changed unsupported paths",
		"grep -Ev '^(.uigraph.yaml|.uigraph/)'",
	} {
		if !strings.Contains(workflow, expected) {
			t.Fatalf("workflow does not contain %q", expected)
		}
	}
}

func TestWorkflowKeepsTheOnboardingTokenAwayFromGeneration(t *testing.T) {
	workflow := string(Workflow("main"))
	var parsed struct {
		Jobs map[string]struct {
			Env   map[string]string `yaml:"env"`
			Steps []struct {
				Name string            `yaml:"name"`
				Run  string            `yaml:"run"`
				Env  map[string]string `yaml:"env"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal([]byte(workflow), &parsed); err != nil {
		t.Fatalf("workflow is invalid YAML: %v", err)
	}
	for name, job := range parsed.Jobs {
		if len(job.Env) != 0 {
			t.Fatalf("job %q declares job-wide env, which would expose the onboarding token to generation: %v", name, job.Env)
		}
		for _, step := range job.Steps {
			_, hasAIKey := step.Env["AI_PROVIDER_API_KEY"]
			_, hasToken := step.Env["UIGRAPH_TOKEN"]
			if hasAIKey && hasToken {
				t.Fatalf("step %q receives both the AI key and the onboarding token", step.Name)
			}
			if hasToken && strings.Contains(step.Run, "@uigraph/agents") {
				t.Fatalf("step %q runs the agent with the onboarding token in scope", step.Name)
			}
		}
	}
}

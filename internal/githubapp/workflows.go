package githubapp

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

const WorkflowPath = ".github/workflows/uigraph.yml"

func Branch(onboardingID string) string {
	return "uigraph/onboarding/" + onboardingID
}

func Seed(orgName, repoName, repoURL, teamName string) ([]byte, error) {
	type named struct {
		Name string `yaml:"name"`
	}
	type repository struct {
		Provider string `yaml:"provider"`
		URL      string `yaml:"url"`
	}
	value := struct {
		Version int   `yaml:"version"`
		Project named `yaml:"project"`
		Service struct {
			Name        string     `yaml:"name"`
			Category    string     `yaml:"category"`
			Description string     `yaml:"description"`
			Repository  repository `yaml:"repository"`
			Ownership   struct {
				Team string `yaml:"team"`
			} `yaml:"ownership"`
		} `yaml:"service"`
	}{}
	value.Version = 1
	value.Project.Name = orgName
	value.Service.Name = HumanizeRepositoryName(repoName)
	value.Service.Category = "service"
	value.Service.Description = fmt.Sprintf("%s service", value.Service.Name)
	value.Service.Repository.Provider = "github"
	value.Service.Repository.URL = repoURL
	value.Service.Ownership.Team = teamName
	return yaml.Marshal(value)
}

func HumanizeRepositoryName(name string) string {
	name = strings.TrimSpace(strings.NewReplacer("-", " ", "_", " ", ".", " ").Replace(name))
	parts := strings.Fields(name)
	for index, part := range parts {
		if part == "api" {
			parts[index] = "API"
			continue
		}
		parts[index] = strings.ToUpper(part[:1]) + part[1:]
	}
	if len(parts) == 0 {
		return "Service"
	}
	return strings.Join(parts, " ")
}

func Workflow(defaultBranch string) []byte {
	return []byte(fmt.Sprintf(`name: UiGraph
on:
  push:
    branches:
      - "uigraph/onboarding/**"
  pull_request:
    branches:
      - %s
permissions:
  contents: write
concurrency:
  group: uigraph-${{ github.workflow }}-${{ github.ref }}
  cancel-in-progress: false
jobs:
  uigraph:
    if: github.event_name == 'push' || !startsWith(github.head_ref, 'uigraph/onboarding/')
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - if: github.event_name == 'push'
        uses: actions/setup-node@v4
        with:
          node-version: 22
      - if: github.event_name == 'push'
        name: Generate repository artifacts
        env:
          AI_PROVIDER_API_KEY: ${{ secrets.AI_PROVIDER_API_KEY || vars.AI_PROVIDER_API_KEY }}
          AI_PROVIDER_MODEL: ${{ secrets.AI_PROVIDER_MODEL || vars.AI_PROVIDER_MODEL }}
          AI_PROVIDER_API_URL: ${{ secrets.AI_PROVIDER_API_URL || vars.AI_PROVIDER_API_URL }}
          AI_PROVIDER_NPM: ${{ secrets.AI_PROVIDER_NPM || vars.AI_PROVIDER_NPM }}
        run: npx --yes @uigraph/agents@experimental artifacts init --seeded
      - if: github.event_name == 'push'
        name: Restrict generated changes
        run: |
          unexpected=$(git status --porcelain | awk '{print $2}' | grep -Ev '^(.uigraph.yaml|.uigraph/)' || true)
          test -z "$unexpected" || { printf 'Generation changed unsupported paths:\n%%s\n' "$unexpected"; exit 1; }
      - uses: actions/setup-go@v5
        with:
          go-version: stable
      - run: go install github.com/uigraph-oss/uigraph-cli@main
      - if: github.event_name == 'push'
        name: Commit generated artifacts
        run: |
          git config user.name "uigraph[bot]"
          git config user.email "uigraph[bot]@users.noreply.github.com"
          git add .uigraph.yaml
          if [ -d .uigraph ]; then
            git add .uigraph
          fi
          if git diff --cached --quiet; then
            echo "Generation produced no changes to commit"
          else
            git commit -m "chore(uigraph): generate repository artifacts"
            git push origin "HEAD:${GITHUB_REF_NAME}"
          fi
      - name: Sync
        env:
          UIGRAPH_TOKEN: ${{ secrets.UIGRAPH_TOKEN }}
          UIGRAPH_GATEWAY_URL: ${{ vars.UIGRAPH_GATEWAY_URL || secrets.UIGRAPH_GATEWAY_URL }}
        run: |
          if [ ! -f .uigraph.yaml ]; then
            echo "No .uigraph.yaml in this ref; skipping sync"
            exit 0
          fi
          uigraph-cli sync
`, defaultBranch))
}

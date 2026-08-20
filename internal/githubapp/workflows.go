package githubapp

import (
	"embed"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed workflows/*.yml
var workflowFiles embed.FS

const (
	ArtifactWorkflowPath   = ".github/workflows/uigraph-artifact.yml"
	SyncWorkflowPath       = ".github/workflows/uigraph-sync.yml"
	OnboardingWorkflowPath = ".github/workflows/uigraph-onboarding.yml"

	EnterpriseArtifactWorkflowPath   = ".github/workflows/uigraph-enterprise-artifact.yml"
	EnterpriseSyncWorkflowPath       = ".github/workflows/uigraph-enterprise-sync.yml"
	EnterpriseOnboardingWorkflowPath = ".github/workflows/uigraph-enterprise-onboarding.yml"

	OnboardingAuthorName  = "uigraph-onboarding[bot]"
	OnboardingAuthorEmail = "uigraph-onboarding[bot]@users.noreply.github.com"
)

func IsOnboardingWorkflowPath(path string) bool {
	return path == OnboardingWorkflowPath || path == EnterpriseOnboardingWorkflowPath
}

func Branch(importID string) string {
	return "uigraph/onboarding/" + importID
}

func Seed(repoName, repoURL, teamName string) ([]byte, error) {
	type repository struct {
		Provider string `yaml:"provider"`
		URL      string `yaml:"url"`
	}
	value := struct {
		Version int `yaml:"version"`
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

func Workflows(defaultBranch string, enterprise bool) (map[string][]byte, error) {
	if defaultBranch == "" {
		return nil, fmt.Errorf("workflows: repository default branch is empty")
	}
	paths := []string{ArtifactWorkflowPath, SyncWorkflowPath, OnboardingWorkflowPath}
	if enterprise {
		paths = []string{EnterpriseArtifactWorkflowPath, EnterpriseSyncWorkflowPath, EnterpriseOnboardingWorkflowPath}
	}
	replacer := strings.NewReplacer("${DEFAULT_BRANCH}", defaultBranch)
	files := make(map[string][]byte, 3)
	for _, path := range paths {
		content, err := workflowFiles.ReadFile("workflows/" + strings.TrimPrefix(path, ".github/workflows/"))
		if err != nil {
			return nil, err
		}
		files[path] = []byte(replacer.Replace(string(content)))
	}
	return files, nil
}

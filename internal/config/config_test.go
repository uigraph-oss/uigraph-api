package config

import (
	"strings"
	"testing"
)

var githubEnvironment = []string{
	"GITHUB_APP_ENABLED", "GITHUB_APP_ID", "GITHUB_APP_SLUG", "GITHUB_APP_CLIENT_ID",
	"GITHUB_APP_CLIENT_SECRET", "GITHUB_APP_PRIVATE_KEY_BASE64", "GITHUB_WEBHOOK_SECRET",
}

func clearGitHubEnvironment(t *testing.T) {
	t.Helper()
	for _, name := range githubEnvironment {
		t.Setenv(name, "")
	}
	t.Setenv("POSTGRES_URL", "postgres://test")
	t.Setenv("UIGRAPH_SECRET_KEY", "secret")
}

func TestLoadRejectsPartialGitHubAppConfiguration(t *testing.T) {
	clearGitHubEnvironment(t)
	t.Setenv("GITHUB_APP_ENABLED", "true")
	t.Setenv("GITHUB_APP_ID", "123")
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "partial GitHub App configuration") {
		t.Fatalf("error = %v", err)
	}
}

func TestLoadRejectsGitHubConfigurationWhenDisabled(t *testing.T) {
	clearGitHubEnvironment(t)
	t.Setenv("GITHUB_APP_ENABLED", "false")
	t.Setenv("GITHUB_APP_ID", "123")
	t.Setenv("GITHUB_APP_SLUG", "uigraph")
	t.Setenv("GITHUB_APP_CLIENT_ID", "client")
	t.Setenv("GITHUB_APP_CLIENT_SECRET", "secret")
	t.Setenv("GITHUB_APP_PRIVATE_KEY_BASE64", "key")
	t.Setenv("GITHUB_WEBHOOK_SECRET", "webhook")
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "GITHUB_APP_ENABLED must be true") {
		t.Fatalf("error = %v", err)
	}
}

func TestLoadAcceptsCompleteGitHubAppConfiguration(t *testing.T) {
	clearGitHubEnvironment(t)
	t.Setenv("GITHUB_APP_ENABLED", "true")
	t.Setenv("GITHUB_APP_ID", "123")
	t.Setenv("GITHUB_APP_SLUG", "uigraph")
	t.Setenv("GITHUB_APP_CLIENT_ID", "client")
	t.Setenv("GITHUB_APP_CLIENT_SECRET", "secret")
	t.Setenv("GITHUB_APP_PRIVATE_KEY_BASE64", "key")
	t.Setenv("GITHUB_WEBHOOK_SECRET", "webhook")
	config, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !config.GitHubAppEnabled || config.GitHubAppID != 123 {
		t.Fatalf("config = %+v", config)
	}
}

func TestLoadAcceptsGitHubAppWithoutWebhook(t *testing.T) {
	clearGitHubEnvironment(t)
	t.Setenv("GITHUB_APP_ENABLED", "true")
	t.Setenv("GITHUB_APP_ID", "123")
	t.Setenv("GITHUB_APP_SLUG", "uigraph")
	t.Setenv("GITHUB_APP_CLIENT_ID", "client")
	t.Setenv("GITHUB_APP_CLIENT_SECRET", "secret")
	t.Setenv("GITHUB_APP_PRIVATE_KEY_BASE64", "key")
	config, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if config.GitHubWebhookSecret != "" {
		t.Fatalf("webhook secret = %q", config.GitHubWebhookSecret)
	}
}

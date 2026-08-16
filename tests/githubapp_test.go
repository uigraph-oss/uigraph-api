package tests

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/uigraph/app/internal/githubapp"
	"github.com/uigraph/app/internal/org"
)

func TestGitHubOnboardingStoreBatchJobsAndDedupe(t *testing.T) {
	ctx := context.Background()
	user, err := testDB.GetUserByEmail(ctx, "admin@localhost")
	if err != nil || user == nil {
		t.Fatalf("admin user: %v", err)
	}
	team := org.Team{ID: uuid.NewString(), OrgID: orgID, Name: "GitHub Onboarding " + uuid.NewString()}
	if err := testDB.CreateTeam(ctx, team); err != nil {
		t.Fatal(err)
	}
	githubInstallationID := time.Now().UnixNano()
	installation := githubapp.Installation{
		OrgID: orgID, GitHubInstallationID: githubInstallationID, AccountID: githubInstallationID,
		AccountLogin: "acme", AccountType: "Organization", TargetType: "Organization", Status: "active",
	}
	repositories := []githubapp.Repository{{
		GitHubID: githubInstallationID + 1, Name: "payments-api", FullName: "acme/payments-api",
		URL: "https://github.com/acme/payments-api", DefaultBranch: "main", Selected: true,
	}}
	if err := testDB.UpsertInstallation(ctx, installation, repositories); err != nil {
		t.Fatal(err)
	}
	storedRepositories, err := testDB.ListRepositories(ctx, orgID)
	if err != nil || len(storedRepositories) == 0 {
		t.Fatalf("repositories=%v err=%v", storedRepositories, err)
	}
	var repositoryID string
	for _, repository := range storedRepositories {
		if repository.GitHubID == repositories[0].GitHubID {
			repositoryID = repository.ID
		}
	}
	if repositoryID == "" {
		t.Fatal("stored repository not found")
	}
	batch, err := testDB.CreateBatch(ctx, orgID, team.ID, team.Name, user.ID, []string{repositoryID})
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.Items) != 1 || batch.Items[0].Status != githubapp.StateSelected {
		t.Fatalf("batch = %+v", batch)
	}
	job, err := testDB.ClaimJob(ctx, "integration-worker", time.Minute)
	if err != nil || job == nil {
		t.Fatalf("job=%+v err=%v", job, err)
	}
	if job.OnboardingID != batch.Items[0].ID || job.Kind != githubapp.JobSetupPR {
		t.Fatalf("job = %+v", job)
	}
	if err := testDB.CompleteJob(ctx, job.ID, "integration-worker"); err != nil {
		t.Fatal(err)
	}
	delivery := uuid.NewString()
	created, err := testDB.RecordWebhook(ctx, delivery, "ping", "", githubInstallationID)
	if err != nil || !created {
		t.Fatalf("first delivery created=%v err=%v", created, err)
	}
	created, err = testDB.RecordWebhook(ctx, delivery, "ping", "", githubInstallationID)
	if err != nil || created {
		t.Fatalf("duplicate delivery created=%v err=%v", created, err)
	}
}

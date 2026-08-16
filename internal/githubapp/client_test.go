package githubapp

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/nacl/box"
)

func testPrivateKey(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	encoded := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	return base64.StdEncoding.EncodeToString(encoded)
}

func TestCreateSetupPullRequestUsesOneAtomicGitCommit(t *testing.T) {
	var mutex sync.Mutex
	var blobPaths []string
	var treePaths []string
	blobContents := map[string]string{}
	blobIndex := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/app/installations/7/access_tokens":
			fmt.Fprint(w, `{"token":"installation-token"}`)
		case r.Method == http.MethodGet && r.URL.Path == "/repos/acme/payments-api/pulls":
			fmt.Fprint(w, `[]`)
		case r.Method == http.MethodGet && r.URL.Path == "/repos/acme/payments-api/git/ref/heads/main":
			fmt.Fprint(w, `{"object":{"sha":"base-commit"}}`)
		case r.Method == http.MethodGet && r.URL.Path == "/repos/acme/payments-api/git/commits/base-commit":
			fmt.Fprint(w, `{"tree":{"sha":"base-tree"}}`)
		case r.Method == http.MethodPost && r.URL.Path == "/repos/acme/payments-api/git/blobs":
			var request struct {
				Content string `json:"content"`
			}
			_ = json.NewDecoder(r.Body).Decode(&request)
			decoded, _ := base64.StdEncoding.DecodeString(request.Content)
			mutex.Lock()
			blobIndex++
			sha := fmt.Sprintf("blob-%d", blobIndex)
			blobContents[sha] = string(decoded)
			mutex.Unlock()
			fmt.Fprintf(w, `{"sha":%q}`, sha)
		case r.Method == http.MethodPost && r.URL.Path == "/repos/acme/payments-api/git/trees":
			var request struct {
				Tree []struct {
					Path string `json:"path"`
					SHA  string `json:"sha"`
				} `json:"tree"`
			}
			_ = json.NewDecoder(r.Body).Decode(&request)
			for _, entry := range request.Tree {
				treePaths = append(treePaths, entry.Path)
				blobPaths = append(blobPaths, entry.SHA)
			}
			fmt.Fprint(w, `{"sha":"new-tree"}`)
		case r.Method == http.MethodPost && r.URL.Path == "/repos/acme/payments-api/git/commits":
			fmt.Fprint(w, `{"sha":"new-commit"}`)
		case r.Method == http.MethodPost && r.URL.Path == "/repos/acme/payments-api/git/refs":
			w.WriteHeader(http.StatusCreated)
			fmt.Fprint(w, `{}`)
		case r.Method == http.MethodPost && r.URL.Path == "/repos/acme/payments-api/pulls":
			w.WriteHeader(http.StatusCreated)
			fmt.Fprint(w, `{"number":12,"html_url":"https://github.test/pr/12"}`)
		default:
			http.Error(w, r.Method+" "+r.URL.String(), http.StatusNotFound)
		}
	}))
	defer server.Close()
	client, err := NewClient(ClientConfig{AppID: 1, PrivateKeyBase64: testPrivateKey(t), APIURL: server.URL, WebURL: server.URL}, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	onboarding := Onboarding{
		ID: "onboarding-id", TeamName: "Platform", SetupBranch: SetupBranch("onboarding-id"),
		Repository: Repository{Name: "payments-api", FullName: "acme/payments-api", URL: "https://github.com/acme/payments-api", DefaultBranch: "main"},
	}
	pull, err := client.CreateSetupPullRequest(context.Background(), 7, onboarding, "Acme")
	if err != nil {
		t.Fatal(err)
	}
	if pull.Number != 12 || len(treePaths) != 3 || len(blobContents) != 3 {
		t.Fatalf("pull=%+v tree paths=%v blob count=%d", pull, treePaths, len(blobContents))
	}
	expected := []string{".github/workflows/uigraph-generate.yml", ".github/workflows/uigraph-sync.yml", ".uigraph.yaml"}
	if strings.Join(treePaths, ",") != strings.Join(expected, ",") {
		t.Fatalf("atomic tree paths = %v", treePaths)
	}
	sort.Strings(blobPaths)
}

func TestMissingAIConfigurationCombinesRepositoryAndOrganizationSettings(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/app/installations/7/access_tokens":
			fmt.Fprint(w, `{"token":"installation-token"}`)
		case "/repos/acme/repo/actions/secrets":
			fmt.Fprint(w, `{"secrets":[{"name":"AI_PROVIDER_API_KEY"}]}`)
		case "/orgs/acme/actions/secrets":
			fmt.Fprint(w, `{"secrets":[]}`)
		case "/repos/acme/repo/actions/variables":
			fmt.Fprint(w, `{"variables":[{"name":"AI_PROVIDER_MODEL"}]}`)
		case "/orgs/acme/actions/variables":
			fmt.Fprint(w, `{"variables":[{"name":"AI_PROVIDER_API_URL"}]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client, err := NewClient(ClientConfig{AppID: 1, PrivateKeyBase64: testPrivateKey(t), APIURL: server.URL, WebURL: server.URL}, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	missing, err := client.MissingAIConfiguration(context.Background(), 7, Repository{FullName: "acme/repo"}, "acme")
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) != 0 {
		t.Fatalf("missing settings = %v", missing)
	}
}

func TestVerifyUserInstallationUsesUserAndAppAccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/user/installations":
			fmt.Fprint(w, `{"installations":[{"id":7}]}`)
		case "/app/installations/7":
			fmt.Fprint(w, `{"id":7,"app_id":1,"account":{"login":"acme"}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client, err := NewClient(ClientConfig{AppID: 1, PrivateKeyBase64: testPrivateKey(t), APIURL: server.URL, WebURL: server.URL}, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	installation, err := client.VerifyUserInstallation(context.Background(), "user-token", 7)
	if err != nil {
		t.Fatal(err)
	}
	if installation.GetID() != 7 || installation.GetAccount().GetLogin() != "acme" {
		t.Fatalf("installation = %+v", installation)
	}
}

func TestGetWorkflowRunFindsRecentlyDispatchedRun(t *testing.T) {
	createdAt := time.Now().UTC().Format(time.RFC3339)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/app/installations/7/access_tokens":
			fmt.Fprint(w, `{"token":"installation-token"}`)
		case "/repos/acme/repo/actions/workflows/uigraph-generate.yml/runs":
			fmt.Fprintf(w, `{"workflow_runs":[{"id":42,"name":"UiGraph Generate","event":"workflow_dispatch","status":"in_progress","head_branch":"main","html_url":"https://github.test/runs/42","created_at":%q}]}`, createdAt)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client, err := NewClient(ClientConfig{AppID: 1, PrivateKeyBase64: testPrivateKey(t), APIURL: server.URL, WebURL: server.URL}, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	run, err := client.GetWorkflowRun(context.Background(), 7, Repository{FullName: "acme/repo", DefaultBranch: "main"}, "uigraph-generate.yml", 0, time.Now().Add(-time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if run.ID != 42 || run.Status != "in_progress" {
		t.Fatalf("run = %+v", run)
	}
}

func TestPutOnboardingSecretUsesSelectedRepositoriesForOrganization(t *testing.T) {
	publicKey, _, err := box.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	var secretRequest struct {
		EncryptedValue        string  `json:"encrypted_value"`
		KeyID                 string  `json:"key_id"`
		Visibility            string  `json:"visibility"`
		SelectedRepositoryIDs []int64 `json:"selected_repository_ids"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/app/installations/7/access_tokens":
			fmt.Fprint(w, `{"token":"installation-token"}`)
		case r.Method == http.MethodGet && r.URL.Path == "/orgs/acme/actions/secrets/public-key":
			fmt.Fprintf(w, `{"key_id":"key-id","key":%q}`, base64.StdEncoding.EncodeToString(publicKey[:]))
		case r.Method == http.MethodPut && r.URL.Path == "/orgs/acme/actions/secrets/UIGRAPH_ONBOARDING_TOKEN":
			_ = json.NewDecoder(r.Body).Decode(&secretRequest)
			w.WriteHeader(http.StatusCreated)
		default:
			http.Error(w, r.Method+" "+r.URL.Path, http.StatusNotFound)
		}
	}))
	defer server.Close()
	client, err := NewClient(ClientConfig{AppID: 1, PrivateKeyBase64: testPrivateKey(t), APIURL: server.URL, WebURL: server.URL}, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	installation := Installation{TargetType: "Organization", AccountType: "Organization", AccountLogin: "acme"}
	repositories := []Repository{{GitHubID: 11}, {GitHubID: 12}}
	if err := client.PutOnboardingSecret(context.Background(), 7, installation, repositories, "plaintext-not-persisted"); err != nil {
		t.Fatal(err)
	}
	if secretRequest.KeyID != "key-id" || secretRequest.EncryptedValue == "" || secretRequest.Visibility != "selected" {
		t.Fatalf("secret request = %+v", secretRequest)
	}
	if fmt.Sprint(secretRequest.SelectedRepositoryIDs) != "[11 12]" {
		t.Fatalf("selected repositories = %v", secretRequest.SelectedRepositoryIDs)
	}
}

func TestPutOnboardingSecretFallsBackToRepositoryForPersonalInstallation(t *testing.T) {
	publicKey, _, err := box.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	putSeen := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/app/installations/7/access_tokens":
			fmt.Fprint(w, `{"token":"installation-token"}`)
		case r.Method == http.MethodGet && r.URL.Path == "/repos/alice/repo/actions/secrets/public-key":
			fmt.Fprintf(w, `{"key_id":"key-id","key":%q}`, base64.StdEncoding.EncodeToString(publicKey[:]))
		case r.Method == http.MethodPut && r.URL.Path == "/repos/alice/repo/actions/secrets/UIGRAPH_ONBOARDING_TOKEN":
			putSeen = true
			w.WriteHeader(http.StatusCreated)
		default:
			http.Error(w, r.Method+" "+r.URL.Path, http.StatusNotFound)
		}
	}))
	defer server.Close()
	client, err := NewClient(ClientConfig{AppID: 1, PrivateKeyBase64: testPrivateKey(t), APIURL: server.URL, WebURL: server.URL}, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	installation := Installation{TargetType: "User", AccountType: "User", AccountLogin: "alice"}
	if err := client.PutOnboardingSecret(context.Background(), 7, installation, []Repository{{FullName: "alice/repo"}}, "token"); err != nil {
		t.Fatal(err)
	}
	if !putSeen {
		t.Fatal("repository secret was not installed")
	}
}

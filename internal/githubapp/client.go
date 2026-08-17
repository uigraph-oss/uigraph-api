package githubapp

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	gh "github.com/google/go-github/v74/github"
	"golang.org/x/crypto/nacl/box"
)

type ClientConfig struct {
	AppID            int64
	Slug             string
	ClientID         string
	ClientSecret     string
	PrivateKeyBase64 string
	APIURL           string
	WebURL           string
	GatewayURL       string
	PublicURL        string
}

type Client struct {
	config ClientConfig
	key    *rsa.PrivateKey
	http   *http.Client
}

type WorkflowRun struct {
	ID         int64  `json:"id"`
	Event      string `json:"event"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	HeadBranch string `json:"head_branch"`
	HTMLURL    string `json:"html_url"`
	Path       string `json:"path"`
}

func NewClient(config ClientConfig, httpClient *http.Client) (*Client, error) {
	decoded, err := base64.StdEncoding.DecodeString(config.PrivateKeyBase64)
	if err != nil {
		return nil, fmt.Errorf("decode GitHub App private key: %w", err)
	}
	block, _ := pem.Decode(decoded)
	if block == nil {
		return nil, errors.New("decode GitHub App private key: PEM block not found")
	}
	var key *rsa.PrivateKey
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err == nil {
		var ok bool
		key, ok = parsed.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("GitHub App private key is not RSA")
		}
	} else {
		key, err = x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse GitHub App private key: %w", err)
		}
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{config: config, key: key, http: httpClient}, nil
}

func (c *Client) AuthorizationURL(state, callback string) string {
	values := url.Values{"client_id": {c.config.ClientID}, "redirect_uri": {callback}, "state": {state}}
	return strings.TrimRight(c.config.WebURL, "/") + "/login/oauth/authorize?" + values.Encode()
}

func (c *Client) InstallationURL(state string) string {
	return strings.TrimRight(c.config.WebURL, "/") + "/apps/" + url.PathEscape(c.config.Slug) + "/installations/new?state=" + url.QueryEscape(state)
}

func (c *Client) ExchangeCode(ctx context.Context, code string) (string, error) {
	payload := map[string]string{"client_id": c.config.ClientID, "client_secret": c.config.ClientSecret, "code": code}
	var response struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	if err := c.webRequest(ctx, http.MethodPost, "/login/oauth/access_token", payload, &response); err != nil {
		return "", err
	}
	if response.Error != "" || response.AccessToken == "" {
		return "", fmt.Errorf("GitHub OAuth exchange failed: %s", response.Error)
	}
	return response.AccessToken, nil
}

func (c *Client) AuthenticatedUserID(ctx context.Context, token string) (int64, error) {
	client, err := c.githubClient(token)
	if err != nil {
		return 0, err
	}
	user, _, err := client.Users.Get(ctx, "")
	if err != nil {
		return 0, err
	}
	return user.GetID(), nil
}

func (c *Client) VerifyUserInstallation(ctx context.Context, token string, installationID int64) (*gh.Installation, error) {
	installations, err := c.userInstallations(ctx, token)
	if err != nil {
		return nil, err
	}
	accessible := false
	for _, installation := range installations {
		if installation.GetID() == installationID {
			accessible = true
			break
		}
	}
	if !accessible {
		return nil, errors.New("GitHub user cannot access the selected installation")
	}
	jwt, err := c.appJWT()
	if err != nil {
		return nil, err
	}
	appClient, err := c.githubClient(jwt)
	if err != nil {
		return nil, err
	}
	var installation gh.Installation
	if err := c.do(ctx, appClient, http.MethodGet, fmt.Sprintf("app/installations/%d", installationID), nil, &installation); err != nil {
		return nil, err
	}
	if installation.GetAppID() != c.config.AppID {
		return nil, errors.New("installation does not belong to configured GitHub App")
	}
	return &installation, nil
}

func (c *Client) FindUserInstallation(ctx context.Context, token string) (*gh.Installation, error) {
	installations, err := c.userInstallations(ctx, token)
	if err != nil {
		return nil, err
	}
	matches := []*gh.Installation{}
	for _, installation := range installations {
		if installation.GetAppID() == c.config.AppID {
			matches = append(matches, installation)
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	return nil, nil
}

func (c *Client) userInstallations(ctx context.Context, token string) ([]*gh.Installation, error) {
	client, err := c.githubClient(token)
	if err != nil {
		return nil, err
	}
	installations := []*gh.Installation{}
	for page := 1; ; page++ {
		var response struct {
			Installations []*gh.Installation `json:"installations"`
		}
		path := fmt.Sprintf("user/installations?per_page=100&page=%d", page)
		if err := c.do(ctx, client, http.MethodGet, path, nil, &response); err != nil {
			return nil, err
		}
		installations = append(installations, response.Installations...)
		if len(response.Installations) < 100 {
			return installations, nil
		}
	}
}

func (c *Client) ListInstallationRepositories(ctx context.Context, installationID int64) ([]Repository, error) {
	client, err := c.installationClient(ctx, installationID)
	if err != nil {
		return nil, err
	}
	var response struct {
		Repositories []*gh.Repository `json:"repositories"`
	}
	if err := c.do(ctx, client, http.MethodGet, "installation/repositories?per_page=100", nil, &response); err != nil {
		return nil, err
	}
	repositories := make([]Repository, 0, len(response.Repositories))
	for _, repository := range response.Repositories {
		repositories = append(repositories, Repository{
			GitHubID: repository.GetID(), Name: repository.GetName(), FullName: repository.GetFullName(),
			URL: repository.GetHTMLURL(), DefaultBranch: repository.GetDefaultBranch(), Private: repository.GetPrivate(),
			Archived: repository.GetArchived(), Selected: true,
		})
	}
	return repositories, nil
}

func (c *Client) DeleteInstallation(ctx context.Context, installationID int64) error {
	jwt, err := c.appJWT()
	if err != nil {
		return err
	}
	client, err := c.githubClient(jwt)
	if err != nil {
		return err
	}
	return c.do(ctx, client, http.MethodDelete, fmt.Sprintf("app/installations/%d", installationID), nil, nil)
}

func (c *Client) StartRun(ctx context.Context, installationID int64, value Import, orgName string) error {
	client, err := c.installationClient(ctx, installationID)
	if err != nil {
		return err
	}
	owner, repo, err := splitRepository(value.Repository.FullName)
	if err != nil {
		return err
	}
	seed, err := Seed(orgName, value.Repository.Name, value.Repository.URL, value.TeamName)
	if err != nil {
		return err
	}
	files, err := Workflows(value.Repository.DefaultBranch)
	if err != nil {
		return err
	}
	files[".uigraph.yaml"] = seed
	return c.createAtomicCommit(ctx, client, owner, repo, value.Repository.DefaultBranch, value.Branch, files)
}

func (c *Client) OpenPullRequest(ctx context.Context, installationID int64, value Import) (string, error) {
	client, err := c.installationClient(ctx, installationID)
	if err != nil {
		return "", err
	}
	owner, repo, err := splitRepository(value.Repository.FullName)
	if err != nil {
		return "", err
	}
	existing, err := c.findPullRequest(ctx, client, owner, repo, value.Branch, value.Repository.DefaultBranch)
	if err != nil {
		return "", err
	}
	if existing != "" {
		return existing, nil
	}
	payload := map[string]string{
		"title": "UiGraph: adopt generated repository artifacts", "head": value.Branch,
		"base": value.Repository.DefaultBranch,
		"body": "This repository is already onboarded to UiGraph. Merging keeps the generated artifacts and enables UiGraph checks on future pull requests.",
	}
	var created struct {
		HTMLURL string `json:"html_url"`
	}
	if err := c.do(ctx, client, http.MethodPost, fmt.Sprintf("repos/%s/%s/pulls", owner, repo), payload, &created); err != nil {
		return "", fmt.Errorf("open import pull request: %w", err)
	}
	return created.HTMLURL, nil
}

func (c *Client) MissingAIConfiguration(ctx context.Context, installationID int64, repository Repository, account string) ([]string, error) {
	client, err := c.installationClient(ctx, installationID)
	if err != nil {
		return nil, err
	}
	owner, repo, err := splitRepository(repository.FullName)
	if err != nil {
		return nil, err
	}
	secrets, err := c.actionsNames(ctx, client, fmt.Sprintf("repos/%s/%s/actions/secrets?per_page=100", owner, repo), "secrets")
	if err != nil {
		return nil, err
	}
	orgSecrets, err := c.actionsNames(ctx, client, fmt.Sprintf("orgs/%s/actions/secrets?per_page=100", account), "secrets")
	if err != nil {
		return nil, err
	}
	variables, err := c.actionsNames(ctx, client, fmt.Sprintf("repos/%s/%s/actions/variables?per_page=100", owner, repo), "variables")
	if err != nil {
		return nil, err
	}
	orgVariables, err := c.actionsNames(ctx, client, fmt.Sprintf("orgs/%s/actions/variables?per_page=100", account), "variables")
	if err != nil {
		return nil, err
	}
	secret := maps.Clone(secrets)
	maps.Copy(secret, orgSecrets)
	configured := maps.Clone(secret)
	maps.Copy(configured, variables)
	maps.Copy(configured, orgVariables)
	missing := make([]string, 0, 4)
	if !secret["AI_PROVIDER_API_KEY"] {
		missing = append(missing, "AI_PROVIDER_API_KEY")
	}
	if !configured["AI_PROVIDER_MODEL"] {
		missing = append(missing, "AI_PROVIDER_MODEL")
	}
	if !configured["AI_PROVIDER_API_URL"] && !configured["AI_PROVIDER_NPM"] {
		missing = append(missing, "AI_PROVIDER_API_URL", "AI_PROVIDER_NPM")
	}
	return missing, nil
}

func (c *Client) actionsNames(ctx context.Context, client *gh.Client, path, key string) (map[string]bool, error) {
	names := map[string]bool{}
	var response map[string]json.RawMessage
	if err := c.do(ctx, client, http.MethodGet, path, nil, &response); err != nil {
		var apiError *gh.ErrorResponse
		if errors.As(err, &apiError) && apiError.Response != nil && apiError.Response.StatusCode == http.StatusNotFound {
			return names, nil
		}
		return nil, err
	}
	raw, ok := response[key]
	if !ok {
		return names, nil
	}
	var values []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, err
	}
	for _, value := range values {
		names[value.Name] = true
	}
	return names, nil
}

func (c *Client) GetWorkflowRun(ctx context.Context, installationID int64, repository Repository, branch string, runID int64) (WorkflowRun, error) {
	client, err := c.installationClient(ctx, installationID)
	if err != nil {
		return WorkflowRun{}, err
	}
	owner, repo, err := splitRepository(repository.FullName)
	if err != nil {
		return WorkflowRun{}, err
	}
	if runID != 0 {
		var run WorkflowRun
		if err := c.do(ctx, client, http.MethodGet, fmt.Sprintf("repos/%s/%s/actions/runs/%d", owner, repo, runID), nil, &run); err != nil {
			return WorkflowRun{}, err
		}
		return run, nil
	}
	query := url.Values{"branch": {branch}, "event": {"push"}, "per_page": {"10"}}
	var response struct {
		WorkflowRuns []WorkflowRun `json:"workflow_runs"`
	}
	if err := c.do(ctx, client, http.MethodGet, fmt.Sprintf("repos/%s/%s/actions/runs?%s", owner, repo, query.Encode()), nil, &response); err != nil {
		return WorkflowRun{}, err
	}
	latest := WorkflowRun{}
	for _, run := range response.WorkflowRuns {
		if run.Path != OnboardingWorkflowPath {
			continue
		}
		if run.ID > latest.ID {
			latest = run
		}
	}
	return latest, nil
}

func (c *Client) ListWorkflowRunJobs(ctx context.Context, installationID int64, repository Repository, runID int64) ([]Step, error) {
	if runID == 0 {
		return nil, nil
	}
	client, err := c.installationClient(ctx, installationID)
	if err != nil {
		return nil, err
	}
	owner, repo, err := splitRepository(repository.FullName)
	if err != nil {
		return nil, err
	}
	var response struct {
		Jobs []*gh.WorkflowJob `json:"jobs"`
	}
	if err := c.do(ctx, client, http.MethodGet, fmt.Sprintf("repos/%s/%s/actions/runs/%d/jobs?per_page=100&filter=latest", owner, repo, runID), nil, &response); err != nil {
		return nil, err
	}
	steps := []Step{}
	for _, job := range response.Jobs {
		steps = append(steps, JobSteps(job.GetID(), job.GetName(), job.Steps)...)
	}
	return steps, nil
}

func JobSteps(jobID int64, jobName string, steps []*gh.TaskStep) []Step {
	out := make([]Step, 0, len(steps))
	for _, step := range steps {
		value := Step{
			JobID: jobID, JobName: jobName, Number: int(step.GetNumber()),
			Name: step.GetName(), Status: step.GetStatus(), Conclusion: step.GetConclusion(),
		}
		if step.StartedAt != nil {
			started := step.StartedAt.Time
			value.StartedAt = &started
		}
		if step.CompletedAt != nil {
			completed := step.CompletedAt.Time
			value.CompletedAt = &completed
		}
		out = append(out, value)
	}
	return out
}

const (
	TokenSecret     = "UIGRAPH_TOKEN"
	GatewayVariable = "UIGRAPH_GATEWAY_URL"
	APIVariable     = "UIGRAPH_API_URL"
)

func (c *Client) PutImportCredentials(ctx context.Context, installationID int64, repository Repository, plaintext string) error {
	if c.config.GatewayURL == "" {
		return fmt.Errorf("githubapp: UIGRAPH_GATEWAY_URL is not configured on this UiGraph instance")
	}
	if c.config.PublicURL == "" {
		return fmt.Errorf("githubapp: UIGRAPH_PUBLIC_URL is not configured on this UiGraph instance")
	}
	values := map[string]string{GatewayVariable: c.config.GatewayURL, APIVariable: c.config.PublicURL}
	client, err := c.installationClient(ctx, installationID)
	if err != nil {
		return err
	}
	owner, repo, err := splitRepository(repository.FullName)
	if err != nil {
		return err
	}
	existingSecrets, err := c.actionsNames(ctx, client, fmt.Sprintf("repos/%s/%s/actions/secrets?per_page=100", owner, repo), "secrets")
	if err != nil {
		return err
	}
	existingVariables, err := c.actionsNames(ctx, client, fmt.Sprintf("repos/%s/%s/actions/variables?per_page=100", owner, repo), "variables")
	if err != nil {
		return err
	}
	if !existingSecrets[TokenSecret] {
		keyID, encrypted, err := c.encryptSecret(ctx, client, fmt.Sprintf("repos/%s/%s/actions/secrets/public-key", owner, repo), plaintext)
		if err != nil {
			return err
		}
		secret := map[string]any{"encrypted_value": encrypted, "key_id": keyID}
		if err := c.do(ctx, client, http.MethodPut, fmt.Sprintf("repos/%s/%s/actions/secrets/%s", owner, repo, TokenSecret), secret, nil); err != nil {
			return err
		}
	}
	for _, name := range []string{GatewayVariable, APIVariable} {
		if existingVariables[name] {
			continue
		}
		variable := map[string]any{"name": name, "value": values[name]}
		if err := c.putVariable(ctx, client, fmt.Sprintf("repos/%s/%s/actions/variables", owner, repo), name, variable); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) putVariable(ctx context.Context, client *gh.Client, collection, name string, payload map[string]any) error {
	err := c.do(ctx, client, http.MethodPatch, collection+"/"+name, payload, nil)
	if err == nil {
		return nil
	}
	var apiError *gh.ErrorResponse
	if errors.As(err, &apiError) && apiError.Response != nil && apiError.Response.StatusCode == http.StatusNotFound {
		return c.do(ctx, client, http.MethodPost, collection, payload, nil)
	}
	return err
}

func (c *Client) createAtomicCommit(ctx context.Context, client *gh.Client, owner, repo, base, branch string, files map[string][]byte) error {
	var ref struct {
		Object struct {
			SHA string `json:"sha"`
		} `json:"object"`
	}
	if err := c.do(ctx, client, http.MethodGet, fmt.Sprintf("repos/%s/%s/git/ref/heads/%s", owner, repo, url.PathEscape(base)), nil, &ref); err != nil {
		return err
	}
	var commit struct {
		Tree struct {
			SHA string `json:"sha"`
		} `json:"tree"`
	}
	if err := c.do(ctx, client, http.MethodGet, fmt.Sprintf("repos/%s/%s/git/commits/%s", owner, repo, ref.Object.SHA), nil, &commit); err != nil {
		return err
	}
	treeEntries := make([]map[string]string, 0, len(files))
	for _, path := range slices.Sorted(maps.Keys(files)) {
		content := files[path]
		var blob struct {
			SHA string `json:"sha"`
		}
		payload := map[string]string{"content": base64.StdEncoding.EncodeToString(content), "encoding": "base64"}
		if err := c.do(ctx, client, http.MethodPost, fmt.Sprintf("repos/%s/%s/git/blobs", owner, repo), payload, &blob); err != nil {
			return err
		}
		treeEntries = append(treeEntries, map[string]string{"path": path, "mode": "100644", "type": "blob", "sha": blob.SHA})
	}
	var tree struct {
		SHA string `json:"sha"`
	}
	if err := c.do(ctx, client, http.MethodPost, fmt.Sprintf("repos/%s/%s/git/trees", owner, repo), map[string]any{"base_tree": commit.Tree.SHA, "tree": treeEntries}, &tree); err != nil {
		return err
	}
	var created struct {
		SHA string `json:"sha"`
	}
	payload := map[string]any{"message": "chore(uigraph): configure repository onboarding", "tree": tree.SHA, "parents": []string{ref.Object.SHA},
		"author": map[string]string{"name": OnboardingAuthorName, "email": OnboardingAuthorEmail}}
	if err := c.do(ctx, client, http.MethodPost, fmt.Sprintf("repos/%s/%s/git/commits", owner, repo), payload, &created); err != nil {
		return err
	}
	err := c.do(ctx, client, http.MethodPost, fmt.Sprintf("repos/%s/%s/git/refs", owner, repo), map[string]string{"ref": "refs/heads/" + branch, "sha": created.SHA}, nil)
	if err == nil {
		return nil
	}
	var apiError *gh.ErrorResponse
	if errors.As(err, &apiError) && apiError.Response != nil && apiError.Response.StatusCode == http.StatusUnprocessableEntity {
		payload := map[string]any{"sha": created.SHA, "force": true}
		return c.do(ctx, client, http.MethodPatch, fmt.Sprintf("repos/%s/%s/git/refs/heads/%s", owner, repo, url.PathEscape(branch)), payload, nil)
	}
	return err
}

func (c *Client) findPullRequest(ctx context.Context, client *gh.Client, owner, repo, branch, base string) (string, error) {
	query := url.Values{"state": {"all"}, "head": {owner + ":" + branch}, "base": {base}, "per_page": {"10"}}
	var pulls []struct {
		HTMLURL string `json:"html_url"`
		Head    struct {
			Ref string `json:"ref"`
		} `json:"head"`
		Base struct {
			Ref string `json:"ref"`
		} `json:"base"`
	}
	if err := c.do(ctx, client, http.MethodGet, fmt.Sprintf("repos/%s/%s/pulls?%s", owner, repo, query.Encode()), nil, &pulls); err != nil {
		return "", err
	}
	for _, pull := range pulls {
		if pull.Head.Ref == branch && pull.Base.Ref == base {
			return pull.HTMLURL, nil
		}
	}
	return "", nil
}

func (c *Client) encryptSecret(ctx context.Context, client *gh.Client, path, plaintext string) (string, string, error) {
	var publicKey struct {
		KeyID string `json:"key_id"`
		Key   string `json:"key"`
	}
	if err := c.do(ctx, client, http.MethodGet, path, nil, &publicKey); err != nil {
		return "", "", err
	}
	decoded, err := base64.StdEncoding.DecodeString(publicKey.Key)
	if err != nil || len(decoded) != 32 {
		return "", "", errors.New("invalid GitHub Actions public key")
	}
	var key [32]byte
	copy(key[:], decoded)
	encrypted, err := box.SealAnonymous(nil, []byte(plaintext), &key, rand.Reader)
	if err != nil {
		return "", "", err
	}
	return publicKey.KeyID, base64.StdEncoding.EncodeToString(encrypted), nil
}

func (c *Client) installationClient(ctx context.Context, installationID int64) (*gh.Client, error) {
	jwt, err := c.appJWT()
	if err != nil {
		return nil, err
	}
	client, err := c.githubClient(jwt)
	if err != nil {
		return nil, err
	}
	var response struct {
		Token string `json:"token"`
	}
	if err := c.do(ctx, client, http.MethodPost, fmt.Sprintf("app/installations/%d/access_tokens", installationID), map[string]any{}, &response); err != nil {
		return nil, err
	}
	return c.githubClient(response.Token)
}

func (c *Client) githubClient(token string) (*gh.Client, error) {
	client := gh.NewClient(c.http).WithAuthToken(token)
	base, err := url.Parse(strings.TrimRight(c.config.APIURL, "/") + "/")
	if err != nil {
		return nil, err
	}
	client.BaseURL = base
	client.UploadURL = base
	return client, nil
}

func (c *Client) appJWT() (string, error) {
	now := time.Now().Unix()
	header, _ := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT"})
	payload, _ := json.Marshal(map[string]int64{"iat": now - 60, "exp": now + 540, "iss": c.config.AppID})
	unsigned := rawBase64(header) + "." + rawBase64(payload)
	digest := sha256.Sum256([]byte(unsigned))
	signature, err := rsa.SignPKCS1v15(rand.Reader, c.key, crypto.SHA256, digest[:])
	if err != nil {
		return "", err
	}
	return unsigned + "." + rawBase64(signature), nil
}

func (c *Client) do(ctx context.Context, client *gh.Client, method, path string, body, result any) error {
	request, err := client.NewRequest(method, path, body)
	if err != nil {
		return err
	}
	_, err = client.Do(ctx, request, result)
	return err
}

func (c *Client) webRequest(ctx context.Context, method, path string, body, result any) error {
	encoded, err := json.Marshal(body)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.config.WebURL, "/")+path, bytes.NewReader(encoded))
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")
	response, err := c.http.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return fmt.Errorf("GitHub request failed with %d: %s", response.StatusCode, strings.TrimSpace(string(data)))
	}
	return json.NewDecoder(response.Body).Decode(result)
}

func splitRepository(fullName string) (string, string, error) {
	owner, repo, ok := strings.Cut(fullName, "/")
	if !ok || owner == "" || repo == "" || strings.Contains(repo, "/") {
		return "", "", fmt.Errorf("invalid GitHub repository name %q", fullName)
	}
	return owner, repo, nil
}

func rawBase64(value []byte) string {
	return base64.RawURLEncoding.EncodeToString(value)
}

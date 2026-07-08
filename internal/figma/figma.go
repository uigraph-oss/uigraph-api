package figma

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	authorizeURL = "https://www.figma.com/oauth"
	tokenURL     = "https://api.figma.com/v1/oauth/token"
	refreshURL   = "https://api.figma.com/v1/oauth/refresh"
	apiBase      = "https://api.figma.com"
	scope        = "file_content:read"
)

var httpClient = &http.Client{Timeout: 15 * time.Second}

type Client struct {
	clientID     string
	clientSecret string
	redirectURI  string
}

func New(clientID, clientSecret, redirectURI string) *Client {
	return &Client{clientID: clientID, clientSecret: clientSecret, redirectURI: redirectURI}
}

func (c *Client) Configured() bool {
	return c.clientID != "" && c.clientSecret != "" && c.redirectURI != ""
}

type Tokens struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int
	UserID       string
}

type NodeInfo struct {
	NodeID   string `json:"nodeId"`
	ImageURL string `json:"imageUrl"`
	Name     string `json:"name"`
}

func (c *Client) AuthorizeURL(state string) string {
	q := url.Values{
		"client_id":     {c.clientID},
		"redirect_uri":  {c.redirectURI},
		"scope":         {scope},
		"state":         {state},
		"response_type": {"code"},
	}
	return authorizeURL + "?" + q.Encode()
}

func (c *Client) ExchangeCode(ctx context.Context, code string) (Tokens, error) {
	form := url.Values{
		"redirect_uri": {c.redirectURI},
		"code":         {code},
		"grant_type":   {"authorization_code"},
	}
	tokens, err := c.tokenRequest(ctx, tokenURL, form)
	if err != nil {
		return Tokens{}, err
	}
	if tokens.UserID == "" {
		return Tokens{}, fmt.Errorf("figma: token response missing user_id_string")
	}
	return tokens, nil
}

func (c *Client) RefreshToken(ctx context.Context, refreshToken string) (Tokens, error) {
	form := url.Values{
		"refresh_token": {refreshToken},
	}
	return c.tokenRequest(ctx, refreshURL, form)
}

func (c *Client) tokenRequest(ctx context.Context, endpoint string, form url.Values) (Tokens, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return Tokens{}, fmt.Errorf("figma: build token request: %w", err)
	}
	basic := base64.StdEncoding.EncodeToString([]byte(c.clientID + ":" + c.clientSecret))
	req.Header.Set("Authorization", "Basic "+basic)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return Tokens{}, fmt.Errorf("figma: token request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return Tokens{}, fmt.Errorf("figma: token endpoint returned %d: %s", resp.StatusCode, body)
	}
	var t struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		UserIDString string `json:"user_id_string"`
	}
	if err := json.Unmarshal(body, &t); err != nil {
		return Tokens{}, fmt.Errorf("figma: decode token response: %w", err)
	}
	if t.AccessToken == "" {
		return Tokens{}, fmt.Errorf("figma: token response missing access_token")
	}
	return Tokens{
		AccessToken:  t.AccessToken,
		RefreshToken: t.RefreshToken,
		ExpiresIn:    t.ExpiresIn,
		UserID:       t.UserIDString,
	}, nil
}

var ErrUnauthorized = fmt.Errorf("figma: unauthorized")

func (c *Client) GetNodeInfo(ctx context.Context, accessToken, fileKey, nodeID string) (NodeInfo, error) {
	var nodesResp struct {
		Nodes map[string]struct {
			Document struct {
				Name string `json:"name"`
			} `json:"document"`
		} `json:"nodes"`
	}
	nodesPath := fmt.Sprintf("/v1/files/%s/nodes?ids=%s", url.PathEscape(fileKey), url.QueryEscape(nodeID))
	if err := c.apiGet(ctx, accessToken, nodesPath, &nodesResp); err != nil {
		return NodeInfo{}, err
	}

	var imagesResp struct {
		Images map[string]string `json:"images"`
		Err    string            `json:"err"`
	}
	imagesPath := fmt.Sprintf("/v1/images/%s?ids=%s&format=jpg&scale=2", url.PathEscape(fileKey), url.QueryEscape(nodeID))
	if err := c.apiGet(ctx, accessToken, imagesPath, &imagesResp); err != nil {
		return NodeInfo{}, err
	}

	name := nodeID
	if n, ok := nodesResp.Nodes[nodeID]; ok && n.Document.Name != "" {
		name = n.Document.Name
	}
	return NodeInfo{
		NodeID:   nodeID,
		ImageURL: imagesResp.Images[nodeID],
		Name:     name,
	}, nil
}

func (c *Client) apiGet(ctx context.Context, accessToken, path string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiBase+path, nil)
	if err != nil {
		return fmt.Errorf("figma: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("figma: request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return ErrUnauthorized
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("figma: api %s returned %d: %s", path, resp.StatusCode, body)
	}
	if err := json.Unmarshal(body, dst); err != nil {
		return fmt.Errorf("figma: decode response: %w", err)
	}
	return nil
}

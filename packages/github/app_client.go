package github

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const defaultAPIBaseURL = "https://api.github.com"

type AppClientConfig struct {
	Slug           string
	AppID          int64
	ClientID       string
	PrivateKeyPath string
	APIBaseURL     string
	HTTPClient     *http.Client
}

type AppClient struct {
	slug       string
	appID      int64
	clientID   string
	apiBaseURL string
	httpClient *http.Client
	privateKey *rsa.PrivateKey
}

type InstallationToken struct {
	Token               string            `json:"token"`
	ExpiresAt           time.Time         `json:"expires_at"`
	Permissions         map[string]string `json:"permissions,omitempty"`
	RepositorySelection string            `json:"repository_selection,omitempty"`
}

type InstallationRepository struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	FullName      string `json:"full_name"`
	Private       bool   `json:"private"`
	DefaultBranch string `json:"default_branch"`
	HTMLURL       string `json:"html_url"`
	Owner         struct {
		Login string `json:"login"`
		Type  string `json:"type"`
	} `json:"owner"`
}

func NewAppClient(cfg AppClientConfig) (*AppClient, error) {
	apiBaseURL := strings.TrimSpace(cfg.APIBaseURL)
	if apiBaseURL == "" {
		apiBaseURL = defaultAPIBaseURL
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}

	client := &AppClient{
		slug:       strings.TrimSpace(cfg.Slug),
		appID:      cfg.AppID,
		clientID:   strings.TrimSpace(cfg.ClientID),
		apiBaseURL: strings.TrimRight(apiBaseURL, "/"),
		httpClient: httpClient,
	}

	privateKeyPath := strings.TrimSpace(cfg.PrivateKeyPath)
	if privateKeyPath == "" {
		return client, nil
	}

	privateKey, err := loadPrivateKey(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("load github app private key: %w", err)
	}

	client.privateKey = privateKey
	return client, nil
}

func (c *AppClient) InstallationURL() (string, error) {
	if c.slug == "" {
		return "", fmt.Errorf("github app slug is not configured")
	}

	return fmt.Sprintf("https://github.com/apps/%s/installations/new", url.PathEscape(c.slug)), nil
}

func (c *AppClient) GenerateAppJWT(now time.Time) (string, error) {
	if c.appID == 0 {
		return "", fmt.Errorf("github app id is not configured")
	}
	if c.privateKey == nil {
		return "", fmt.Errorf("github app private key is not configured")
	}

	header := map[string]string{
		"alg": "RS256",
		"typ": "JWT",
	}

	payload := map[string]any{
		"iat": now.Add(-30 * time.Second).Unix(),
		"exp": now.Add(9 * time.Minute).Unix(),
		"iss": c.appID,
	}

	unsigned, err := encodeJWTParts(header, payload)
	if err != nil {
		return "", fmt.Errorf("encode github app jwt: %w", err)
	}

	digest := sha256.Sum256([]byte(unsigned))
	signature, err := rsa.SignPKCS1v15(rand.Reader, c.privateKey, crypto.SHA256, digest[:])
	if err != nil {
		return "", fmt.Errorf("sign github app jwt: %w", err)
	}

	return unsigned + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func (c *AppClient) CreateInstallationToken(ctx context.Context, installationID int64) (InstallationToken, error) {
	if installationID == 0 {
		return InstallationToken{}, fmt.Errorf("installation id is required")
	}

	appJWT, err := c.GenerateAppJWT(time.Now().UTC())
	if err != nil {
		return InstallationToken{}, err
	}

	endpoint := fmt.Sprintf("%s/app/installations/%d/access_tokens", c.apiBaseURL, installationID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, http.NoBody)
	if err != nil {
		return InstallationToken{}, fmt.Errorf("build installation token request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+appJWT)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return InstallationToken{}, fmt.Errorf("request installation token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return InstallationToken{}, decodeGitHubError(resp)
	}

	var token InstallationToken
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return InstallationToken{}, fmt.Errorf("decode installation token response: %w", err)
	}

	return token, nil
}

func (c *AppClient) ListInstallationRepositories(ctx context.Context, installationID int64) ([]InstallationRepository, error) {
	token, err := c.CreateInstallationToken(ctx, installationID)
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("%s/installation/repositories", c.apiBaseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("build installation repositories request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+token.Token)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request installation repositories: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, decodeGitHubError(resp)
	}

	var payload struct {
		Repositories []InstallationRepository `json:"repositories"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode installation repositories response: %w", err)
	}

	return payload.Repositories, nil
}

func loadPrivateKey(path string) (*rsa.PrivateKey, error) {
	contents, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("read private key file: %w", err)
	}

	block, _ := pem.Decode(contents)
	if block == nil {
		return nil, fmt.Errorf("decode pem block: invalid private key format")
	}

	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}

	privateKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}

	rsaKey, ok := privateKey.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("parse private key: unsupported key type")
	}

	return rsaKey, nil
}

func encodeJWTParts(header map[string]string, payload map[string]any) (string, error) {
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(headerJSON) + "." + base64.RawURLEncoding.EncodeToString(payloadJSON), nil
}

func decodeGitHubError(resp *http.Response) error {
	var payload struct {
		Message          string `json:"message"`
		DocumentationURL string `json:"documentation_url"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return fmt.Errorf("github api status %d", resp.StatusCode)
	}

	message := strings.TrimSpace(payload.Message)
	if message == "" {
		message = fmt.Sprintf("github api status %d", resp.StatusCode)
	}

	if strings.TrimSpace(payload.DocumentationURL) != "" {
		return fmt.Errorf("%s (%s)", message, payload.DocumentationURL)
	}

	return fmt.Errorf("%s", message)
}

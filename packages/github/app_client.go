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
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"
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

type RepositoryContributor struct {
	ID            int64   `json:"id"`
	Login         string  `json:"login"`
	AvatarURL     string  `json:"avatar_url"`
	Contributions int     `json:"contributions"`
	Type          string  `json:"type"`
	Name          *string `json:"name,omitempty"`
}

type RepositoryCommitFile struct {
	Path         string  `json:"path"`
	PreviousPath *string `json:"previous_path,omitempty"`
	ChangeType   string  `json:"change_type"`
	Additions    int     `json:"additions"`
	Deletions    int     `json:"deletions"`
	Changes      int     `json:"changes"`
	PatchText    *string `json:"patch_text,omitempty"`
}

type RepositoryCommit struct {
	SHA                string                 `json:"sha"`
	AuthorGitHubUserID *int64                 `json:"author_github_user_id,omitempty"`
	AuthorUsername     string                 `json:"author_username,omitempty"`
	AuthorName         string                 `json:"author_name,omitempty"`
	AuthorEmail        string                 `json:"author_email,omitempty"`
	CommittedAt        time.Time              `json:"committed_at"`
	Message            string                 `json:"message,omitempty"`
	ParentCount        int                    `json:"parent_count"`
	Additions          int                    `json:"additions"`
	Deletions          int                    `json:"deletions"`
	TotalChanges       int                    `json:"total_changes"`
	Files              []RepositoryCommitFile `json:"files"`
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

func (c *AppClient) ListRepositoryContributors(ctx context.Context, installationID int64, owner string, repo string) ([]RepositoryContributor, error) {
	token, err := c.CreateInstallationToken(ctx, installationID)
	if err != nil {
		return nil, err
	}

	const perPage = 100

	allContributors := make([]RepositoryContributor, 0, perPage)
	for page := 1; ; page++ {
		endpoint := fmt.Sprintf(
			"%s/repos/%s/%s/contributors?per_page=%d&page=%d",
			c.apiBaseURL,
			url.PathEscape(owner),
			url.PathEscape(repo),
			perPage,
			page,
		)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
		if err != nil {
			return nil, fmt.Errorf("build repository contributors request: %w", err)
		}

		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("Authorization", "Bearer "+token.Token)
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("request repository contributors page %d: %w", page, err)
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			defer resp.Body.Close()
			return nil, decodeGitHubError(resp)
		}

		var contributors []RepositoryContributor
		if err := json.NewDecoder(resp.Body).Decode(&contributors); err != nil {
			if errors.Is(err, io.EOF) {
				resp.Body.Close()
				break
			}
			resp.Body.Close()
			return nil, fmt.Errorf("decode repository contributors response page %d: %w", page, err)
		}
		resp.Body.Close()

		allContributors = append(allContributors, contributors...)
		if len(contributors) < perPage {
			break
		}
	}

	return allContributors, nil
}

func (c *AppClient) ListRepositoryCommits(ctx context.Context, installationID int64, owner string, repo string) ([]RepositoryCommit, error) {
	token, err := c.CreateInstallationToken(ctx, installationID)
	if err != nil {
		return nil, err
	}

	const perPage = 100
	const maxAttempts = 3

	type commitSummary struct {
		SHA string `json:"sha"`
	}

	summaries := make([]commitSummary, 0, perPage)
	for page := 1; ; page++ {
		var pageSummaries []commitSummary
		for attempt := 1; ; attempt++ {
			endpoint := fmt.Sprintf(
				"%s/repos/%s/%s/commits?per_page=%d&page=%d",
				c.apiBaseURL,
				url.PathEscape(owner),
				url.PathEscape(repo),
				perPage,
				page,
			)
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
			if err != nil {
				return nil, fmt.Errorf("build repository commits request: %w", err)
			}

			req.Header.Set("Accept", "application/vnd.github+json")
			req.Header.Set("Authorization", "Bearer "+token.Token)
			req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

			resp, err := c.httpClient.Do(req)
			if err != nil {
				return nil, fmt.Errorf("request repository commits page %d: %w", page, err)
			}

			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				if attempt < maxAttempts && isRetryableGitHubStatus(resp.StatusCode) {
					resp.Body.Close()
					if err := sleepWithContext(ctx, time.Duration(attempt)*500*time.Millisecond); err != nil {
						return nil, err
					}
					continue
				}

				defer resp.Body.Close()
				return nil, decodeGitHubError(resp)
			}

			if err := json.NewDecoder(resp.Body).Decode(&pageSummaries); err != nil {
				resp.Body.Close()
				return nil, fmt.Errorf("decode repository commits response page %d: %w", page, err)
			}
			resp.Body.Close()
			break
		}

		summaries = append(summaries, pageSummaries...)
		if len(pageSummaries) < perPage {
			break
		}
	}

	commits := make([]RepositoryCommit, len(summaries))
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(8)

	for index, summary := range summaries {
		index := index
		summary := summary

		group.Go(func() error {
			commit, err := c.getRepositoryCommit(groupCtx, token.Token, owner, repo, summary.SHA)
			if err != nil {
				return err
			}

			// Preserve summary order even though detail fetches run concurrently.
			commits[index] = commit
			return nil
		})
	}

	if err := group.Wait(); err != nil {
		return nil, err
	}

	return commits, nil
}

func (c *AppClient) getRepositoryCommit(ctx context.Context, installationToken string, owner string, repo string, sha string) (RepositoryCommit, error) {
	const maxAttempts = 3

	var payload struct {
		SHA    string `json:"sha"`
		Author *struct {
			ID    int64  `json:"id"`
			Login string `json:"login"`
		} `json:"author"`
		Commit struct {
			Author struct {
				Name  string    `json:"name"`
				Email string    `json:"email"`
				Date  time.Time `json:"date"`
			} `json:"author"`
			Message string `json:"message"`
		} `json:"commit"`
		Parents []struct {
			SHA string `json:"sha"`
		} `json:"parents"`
		Stats struct {
			Additions int `json:"additions"`
			Deletions int `json:"deletions"`
			Total     int `json:"total"`
		} `json:"stats"`
		Files []struct {
			Filename         string `json:"filename"`
			PreviousFilename string `json:"previous_filename"`
			Status           string `json:"status"`
			Additions        int    `json:"additions"`
			Deletions        int    `json:"deletions"`
			Changes          int    `json:"changes"`
			Patch            string `json:"patch"`
		} `json:"files"`
	}

	for attempt := 1; ; attempt++ {
		endpoint := fmt.Sprintf(
			"%s/repos/%s/%s/commits/%s",
			c.apiBaseURL,
			url.PathEscape(owner),
			url.PathEscape(repo),
			url.PathEscape(sha),
		)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
		if err != nil {
			return RepositoryCommit{}, fmt.Errorf("build repository commit detail request: %w", err)
		}

		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("Authorization", "Bearer "+installationToken)
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return RepositoryCommit{}, fmt.Errorf("request repository commit detail %s: %w", sha, err)
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			if attempt < maxAttempts && isRetryableGitHubStatus(resp.StatusCode) {
				resp.Body.Close()
				if err := sleepWithContext(ctx, time.Duration(attempt)*500*time.Millisecond); err != nil {
					return RepositoryCommit{}, err
				}
				continue
			}

			defer resp.Body.Close()
			return RepositoryCommit{}, decodeGitHubError(resp)
		}

		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			resp.Body.Close()
			return RepositoryCommit{}, fmt.Errorf("decode repository commit detail %s: %w", sha, err)
		}
		resp.Body.Close()
		break
	}

	commit := RepositoryCommit{
		SHA:          payload.SHA,
		AuthorName:   payload.Commit.Author.Name,
		AuthorEmail:  payload.Commit.Author.Email,
		CommittedAt:  payload.Commit.Author.Date,
		Message:      payload.Commit.Message,
		ParentCount:  len(payload.Parents),
		Additions:    payload.Stats.Additions,
		Deletions:    payload.Stats.Deletions,
		TotalChanges: payload.Stats.Total,
		Files:        make([]RepositoryCommitFile, 0, len(payload.Files)),
	}
	if payload.Author != nil {
		commit.AuthorGitHubUserID = &payload.Author.ID
		commit.AuthorUsername = payload.Author.Login
	}

	for _, file := range payload.Files {
		var previousPath *string
		if strings.TrimSpace(file.PreviousFilename) != "" {
			previousPath = &file.PreviousFilename
		}

		var patchText *string
		if strings.TrimSpace(file.Patch) != "" {
			patchText = &file.Patch
		}

		changeType := normalizeGitHubFileStatus(file.Status)
		commit.Files = append(commit.Files, RepositoryCommitFile{
			Path:         file.Filename,
			PreviousPath: previousPath,
			ChangeType:   changeType,
			Additions:    file.Additions,
			Deletions:    file.Deletions,
			Changes:      file.Changes,
			PatchText:    patchText,
		})
	}

	return commit, nil
}

func isRetryableGitHubStatus(statusCode int) bool {
	return statusCode >= 500 && statusCode <= 599
}

func sleepWithContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func normalizeGitHubFileStatus(status string) string {
	switch strings.TrimSpace(strings.ToLower(status)) {
	case "removed":
		return "deleted"
	default:
		return strings.TrimSpace(strings.ToLower(status))
	}
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

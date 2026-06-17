package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	githubAuthorizeURL   = "https://github.com/login/oauth/authorize"
	githubTokenURL       = "https://github.com/login/oauth/access_token"
	githubCurrentUserURL = "https://api.github.com/user"
)

type GitHubClient struct {
	client       *http.Client
	clientID     string
	clientSecret string
	redirectURL  string
	prompt       string
}

type GitHubUser struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	AvatarURL string `json:"avatar_url"`
}

type accessTokenResponse struct {
	AccessToken string `json:"access_token"`
}

func NewGitHubClient(clientID string, clientSecret string, redirectURL string, prompt string) *GitHubClient {
	return &GitHubClient{
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURL:  redirectURL,
		prompt:       prompt,
	}
}

func (c *GitHubClient) BuildAuthorizeURL(state string) string {
	query := url.Values{}
	query.Set("client_id", c.clientID)
	query.Set("redirect_uri", c.redirectURL)
	query.Set("scope", "read:user")
	query.Set("state", state)
	if strings.TrimSpace(c.prompt) != "" {
		query.Set("prompt", c.prompt)
	}

	return githubAuthorizeURL + "?" + query.Encode()
}

func (c *GitHubClient) ExchangeCode(ctx context.Context, code string) (string, error) {
	form := url.Values{}
	form.Set("client_id", c.clientID)
	form.Set("client_secret", c.clientSecret)
	form.Set("redirect_uri", c.redirectURL)
	form.Set("code", code)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, githubTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("build token exchange request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("exchange github code: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github token exchange failed with status %d", resp.StatusCode)
	}

	var tokenResp accessTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("decode github token response: %w", err)
	}

	if tokenResp.AccessToken == "" {
		return "", fmt.Errorf("github access token missing from response")
	}

	return tokenResp.AccessToken, nil
}

func (c *GitHubClient) FetchUser(ctx context.Context, accessToken string) (GitHubUser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubCurrentUserURL, nil)
	if err != nil {
		return GitHubUser{}, fmt.Errorf("build github user request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.client.Do(req)
	if err != nil {
		return GitHubUser{}, fmt.Errorf("fetch github user: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return GitHubUser{}, fmt.Errorf("github user fetch failed with status %d", resp.StatusCode)
	}

	var user GitHubUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return GitHubUser{}, fmt.Errorf("decode github user response: %w", err)
	}

	return user, nil
}

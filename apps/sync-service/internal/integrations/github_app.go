package integrations

import (
	"context"
	"fmt"
	"net/http"
	"time"

	sharedgithub "codeatlas/packages/github"
)

type GitHubAppConfig struct {
	Slug           string
	AppID          int64
	ClientID       string
	PrivateKeyPath string
	APIBaseURL     string
	APITimeout     time.Duration
}

type GitHubApp struct {
	client *sharedgithub.AppClient
}

func NewGitHubApp(cfg GitHubAppConfig) (*GitHubApp, error) {
	client, err := sharedgithub.NewAppClient(sharedgithub.AppClientConfig{
		Slug:           cfg.Slug,
		AppID:          cfg.AppID,
		ClientID:       cfg.ClientID,
		PrivateKeyPath: cfg.PrivateKeyPath,
		APIBaseURL:     cfg.APIBaseURL,
		HTTPClient:     &http.Client{Timeout: cfg.APITimeout},
	})
	if err != nil {
		return nil, err
	}

	return &GitHubApp{client: client}, nil
}

func (g *GitHubApp) ListRepositoryContributors(ctx context.Context, installationID int64, owner string, repo string) ([]sharedgithub.RepositoryContributor, error) {
	contributors, err := g.client.ListRepositoryContributors(ctx, installationID, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("list repository contributors: %w", err)
	}

	return contributors, nil
}

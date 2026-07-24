package integrations

import (
	"context"
	"fmt"
	"time"

	sharedgithub "codeatlas/packages/github"
)

type GitHubAppConfig struct {
	Slug           string
	AppID          int64
	ClientID       string
	PrivateKeyPath string
	APIBaseURL     string
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
	})
	if err != nil {
		return nil, err
	}

	return &GitHubApp{client: client}, nil
}

func (g *GitHubApp) InstallationURL() (string, error) {
	return g.client.InstallationURL()
}

func (g *GitHubApp) GenerateAppJWT() (string, error) {
	return g.client.GenerateAppJWT(time.Now().UTC())
}

func (g *GitHubApp) CreateInstallationToken(ctx context.Context, installationID int64) (sharedgithub.InstallationToken, error) {
	token, err := g.client.CreateInstallationToken(ctx, installationID)
	if err != nil {
		return sharedgithub.InstallationToken{}, fmt.Errorf("create installation token: %w", err)
	}

	return token, nil
}

func (g *GitHubApp) ListInstallationRepositories(ctx context.Context, installationID int64) ([]sharedgithub.InstallationRepository, error) {
	repositories, err := g.client.ListInstallationRepositories(ctx, installationID)
	if err != nil {
		return nil, fmt.Errorf("list installation repositories: %w", err)
	}

	return repositories, nil
}

func (g *GitHubApp) ListRepositoryWebhooks(ctx context.Context, installationID int64, owner string, repo string) ([]sharedgithub.RepositoryWebhook, error) {
	webhooks, err := g.client.ListRepositoryWebhooks(ctx, installationID, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("list repository webhooks: %w", err)
	}

	return webhooks, nil
}

func (g *GitHubApp) CreateRepositoryWebhook(ctx context.Context, installationID int64, owner string, repo string, input sharedgithub.RepositoryWebhookInput) (sharedgithub.RepositoryWebhook, error) {
	webhook, err := g.client.CreateRepositoryWebhook(ctx, installationID, owner, repo, input)
	if err != nil {
		return sharedgithub.RepositoryWebhook{}, fmt.Errorf("create repository webhook: %w", err)
	}

	return webhook, nil
}

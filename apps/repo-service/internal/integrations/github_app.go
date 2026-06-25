package integrations

import (
	"fmt"
	"net/url"
	"strings"
)

type GitHubApp struct {
	slug string
}

func NewGitHubApp(slug string) *GitHubApp {
	return &GitHubApp{slug: strings.TrimSpace(slug)}
}

func (g *GitHubApp) InstallationURL() (string, error) {
	if g.slug == "" {
		return "", fmt.Errorf("github app slug is not configured")
	}

	return fmt.Sprintf("https://github.com/apps/%s/installations/new", url.PathEscape(g.slug)), nil
}

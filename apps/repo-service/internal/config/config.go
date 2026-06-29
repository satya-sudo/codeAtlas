package config

import (
	"time"

	sharedconfig "codeatlas/packages/config"
)

type Config struct {
	ServiceName                    string
	AppEnv                         string
	LogLevel                       string
	LogJSON                        bool
	HTTPPort                       int
	ShutdownTimeout                time.Duration
	FrontendOrigin                 string
	FrontendGitHubSetupRedirectURL string
	JWTSecret                      string
	GitHubAppSlug                  string
	GitHubAppID                    int64
	GitHubAppClientID              string
	GitHubAppPrivateKeyPath        string
	GitHubAPIBaseURL               string
}

func Load() (Config, error) {
	logJSON, err := sharedconfig.GetBool("LOG_JSON", false)
	if err != nil {
		return Config{}, err
	}

	httpPort, err := sharedconfig.GetInt("REPO_SERVICE_PORT", 8062)
	if err != nil {
		return Config{}, err
	}

	shutdownTimeout, err := sharedconfig.GetDuration("REPO_SERVICE_SHUTDOWN_TIMEOUT", 10*time.Second)
	if err != nil {
		return Config{}, err
	}

	jwtSecret, err := sharedconfig.MustString("JWT_SECRET")
	if err != nil {
		return Config{}, err
	}

	githubAppID, err := sharedconfig.GetInt("GITHUB_APP_ID", 0)
	if err != nil {
		return Config{}, err
	}

	return Config{
		ServiceName:                    "repo-service",
		AppEnv:                         sharedconfig.GetString("APP_ENV", "development"),
		LogLevel:                       sharedconfig.GetString("LOG_LEVEL", "info"),
		LogJSON:                        logJSON,
		HTTPPort:                       httpPort,
		ShutdownTimeout:                shutdownTimeout,
		FrontendOrigin:                 sharedconfig.GetString("FRONTEND_ORIGIN", "http://localhost:6060"),
		FrontendGitHubSetupRedirectURL: sharedconfig.GetString("FRONTEND_GITHUB_SETUP_REDIRECT_URL", "http://localhost:6060/github/installations/callback"),
		JWTSecret:                      jwtSecret,
		GitHubAppSlug:                  sharedconfig.GetString("GITHUB_APP_SLUG", ""),
		GitHubAppID:                    int64(githubAppID),
		GitHubAppClientID:              sharedconfig.GetString("GITHUB_APP_CLIENT_ID", ""),
		GitHubAppPrivateKeyPath:        sharedconfig.GetString("GITHUB_APP_PRIVATE_KEY_PATH", ""),
		GitHubAPIBaseURL:               sharedconfig.GetString("GITHUB_API_BASE_URL", "https://api.github.com"),
	}, nil
}

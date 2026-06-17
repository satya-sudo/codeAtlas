package config

import (
	"time"

	sharedconfig "codeatlas/packages/config"
)

type Config struct {
	ServiceName         string
	AppEnv              string
	LogLevel            string
	LogJSON             bool
	HTTPPort            int
	ShutdownTimeout     time.Duration
	GitHubClientID      string
	GitHubClientSecret  string
	GitHubRedirectURL   string
	GitHubPrompt        string
	FrontendRedirectURL string
	FrontendOrigin      string
	JWTSecret           string
	JWTTTL              time.Duration
	StateCookieName     string
	StateCookieTTL      time.Duration
}

func Load() (Config, error) {
	logJSON, err := sharedconfig.GetBool("LOG_JSON", false)
	if err != nil {
		return Config{}, err
	}

	httpPort, err := sharedconfig.GetInt("AUTH_SERVICE_PORT", 8061)
	if err != nil {
		return Config{}, err
	}

	shutdownTimeout, err := sharedconfig.GetDuration("AUTH_SERVICE_SHUTDOWN_TIMEOUT", 10*time.Second)
	if err != nil {
		return Config{}, err
	}

	jwtTTL, err := sharedconfig.GetDuration("JWT_TTL", 24*time.Hour)
	if err != nil {
		return Config{}, err
	}

	stateCookieTTL, err := sharedconfig.GetDuration("AUTH_STATE_COOKIE_TTL", 10*time.Minute)
	if err != nil {
		return Config{}, err
	}

	githubClientID, err := sharedconfig.MustString("GITHUB_CLIENT_ID")
	if err != nil {
		return Config{}, err
	}

	githubClientSecret, err := sharedconfig.MustString("GITHUB_CLIENT_SECRET")
	if err != nil {
		return Config{}, err
	}

	githubRedirectURL, err := sharedconfig.MustString("GITHUB_REDIRECT_URL")
	if err != nil {
		return Config{}, err
	}

	jwtSecret, err := sharedconfig.MustString("JWT_SECRET")
	if err != nil {
		return Config{}, err
	}

	return Config{
		ServiceName:         "auth-service",
		AppEnv:              sharedconfig.GetString("APP_ENV", "development"),
		LogLevel:            sharedconfig.GetString("LOG_LEVEL", "info"),
		LogJSON:             logJSON,
		HTTPPort:            httpPort,
		ShutdownTimeout:     shutdownTimeout,
		GitHubClientID:      githubClientID,
		GitHubClientSecret:  githubClientSecret,
		GitHubRedirectURL:   githubRedirectURL,
		GitHubPrompt:        sharedconfig.GetString("GITHUB_PROMPT", "select_account"),
		FrontendRedirectURL: sharedconfig.GetString("FRONTEND_REDIRECT_URL", ""),
		FrontendOrigin:      sharedconfig.GetString("FRONTEND_ORIGIN", "http://localhost:6060"),
		JWTSecret:           jwtSecret,
		JWTTTL:              jwtTTL,
		StateCookieName:     sharedconfig.GetString("AUTH_STATE_COOKIE_NAME", "codeatlas_auth_state"),
		StateCookieTTL:      stateCookieTTL,
	}, nil
}

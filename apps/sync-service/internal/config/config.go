package config

import (
	"time"

	sharedconfig "codeatlas/packages/config"
)

type Config struct {
	ServiceName      string
	AppEnv           string
	LogLevel         string
	LogJSON          bool
	PollInterval     time.Duration
	ShutdownTimeout  time.Duration
	GitHubAPITimeout time.Duration
}

func Load() (Config, error) {
	logJSON, err := sharedconfig.GetBool("LOG_JSON", false)
	if err != nil {
		return Config{}, err
	}

	pollInterval, err := sharedconfig.GetDuration("SYNC_SERVICE_POLL_INTERVAL", 30*time.Second)
	if err != nil {
		return Config{}, err
	}

	shutdownTimeout, err := sharedconfig.GetDuration("SYNC_SERVICE_SHUTDOWN_TIMEOUT", 10*time.Second)
	if err != nil {
		return Config{}, err
	}

	githubTimeout, err := sharedconfig.GetDuration("GITHUB_API_TIMEOUT", 15*time.Second)
	if err != nil {
		return Config{}, err
	}

	return Config{
		ServiceName:      "sync-service",
		AppEnv:           sharedconfig.GetString("APP_ENV", "development"),
		LogLevel:         sharedconfig.GetString("LOG_LEVEL", "info"),
		LogJSON:          logJSON,
		PollInterval:     pollInterval,
		ShutdownTimeout:  shutdownTimeout,
		GitHubAPITimeout: githubTimeout,
	}, nil
}

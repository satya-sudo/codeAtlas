package config

import (
	"time"

	sharedconfig "codeatlas/packages/config"
)

type Config struct {
	ServiceName     string
	AppEnv          string
	LogLevel        string
	LogJSON         bool
	HTTPPort        int
	ShutdownTimeout time.Duration
}

func Load() (Config, error) {
	logJSON, err := sharedconfig.GetBool("LOG_JSON", false)
	if err != nil {
		return Config{}, err
	}

	httpPort, err := sharedconfig.GetInt("WEBHOOK_SERVICE_PORT", 8063)
	if err != nil {
		return Config{}, err
	}

	shutdownTimeout, err := sharedconfig.GetDuration("WEBHOOK_SERVICE_SHUTDOWN_TIMEOUT", 10*time.Second)
	if err != nil {
		return Config{}, err
	}

	return Config{
		ServiceName:     "webhook-service",
		AppEnv:          sharedconfig.GetString("APP_ENV", "development"),
		LogLevel:        sharedconfig.GetString("LOG_LEVEL", "info"),
		LogJSON:         logJSON,
		HTTPPort:        httpPort,
		ShutdownTimeout: shutdownTimeout,
	}, nil
}

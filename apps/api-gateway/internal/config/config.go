package config

import (
	"time"

	sharedconfig "codeatlas/packages/config"
)

type Config struct {
	ServiceName       string
	AppEnv            string
	LogLevel          string
	LogJSON           bool
	HTTPPort          int
	ShutdownTimeout   time.Duration
	FrontendOrigin    string
	AuthServiceURL    string
	RepoServiceURL    string
	WebhookServiceURL string
}

func Load() (Config, error) {
	logJSON, err := sharedconfig.GetBool("LOG_JSON", false)
	if err != nil {
		return Config{}, err
	}

	httpPort, err := sharedconfig.GetInt("API_GATEWAY_PORT", 8060)
	if err != nil {
		return Config{}, err
	}

	shutdownTimeout, err := sharedconfig.GetDuration("API_GATEWAY_SHUTDOWN_TIMEOUT", 10*time.Second)
	if err != nil {
		return Config{}, err
	}

	return Config{
		ServiceName:       "api-gateway",
		AppEnv:            sharedconfig.GetString("APP_ENV", "development"),
		LogLevel:          sharedconfig.GetString("LOG_LEVEL", "info"),
		LogJSON:           logJSON,
		HTTPPort:          httpPort,
		ShutdownTimeout:   shutdownTimeout,
		FrontendOrigin:    sharedconfig.GetString("FRONTEND_ORIGIN", "http://localhost:6060"),
		AuthServiceURL:    sharedconfig.GetString("AUTH_SERVICE_BASE_URL", "http://localhost:8061"),
		RepoServiceURL:    sharedconfig.GetString("REPO_SERVICE_BASE_URL", "http://localhost:8062"),
		WebhookServiceURL: sharedconfig.GetString("WEBHOOK_SERVICE_BASE_URL", "http://localhost:8063"),
	}, nil
}

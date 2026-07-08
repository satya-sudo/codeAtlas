package config

import (
	"strings"
	"time"

	sharedconfig "codeatlas/packages/config"
	"codeatlas/packages/events"
	sharedkafka "codeatlas/packages/kafka"
)

type Config struct {
	ServiceName                 string
	AppEnv                      string
	LogLevel                    string
	LogJSON                     bool
	PollInterval                time.Duration
	ShutdownTimeout             time.Duration
	GitHubAPITimeout            time.Duration
	KafkaEnabled                bool
	KafkaBrokers                []string
	RepositorySyncTopic         string
	RepositorySyncConsumerGroup string
	GitHubAppSlug               string
	GitHubAppID                 int64
	GitHubAppClientID           string
	GitHubAppPrivateKeyPath     string
	GitHubAPIBaseURL            string
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

	kafkaEnabled, err := sharedconfig.GetBool("SYNC_SERVICE_KAFKA_ENABLED", false)
	if err != nil {
		return Config{}, err
	}

	var kafkaBrokers []string
	kafkaBrokersRaw := strings.TrimSpace(sharedconfig.GetString("KAFKA_BROKERS", "localhost:9092"))
	if kafkaBrokersRaw != "" {
		kafkaBrokers, err = sharedkafka.ParseBrokers(kafkaBrokersRaw)
		if err != nil {
			return Config{}, err
		}
	}

	githubAppID, err := sharedconfig.GetInt("GITHUB_APP_ID", 0)
	if err != nil {
		return Config{}, err
	}

	return Config{
		ServiceName:                 "sync-service",
		AppEnv:                      sharedconfig.GetString("APP_ENV", "development"),
		LogLevel:                    sharedconfig.GetString("LOG_LEVEL", "info"),
		LogJSON:                     logJSON,
		PollInterval:                pollInterval,
		ShutdownTimeout:             shutdownTimeout,
		GitHubAPITimeout:            githubTimeout,
		KafkaEnabled:                kafkaEnabled,
		KafkaBrokers:                kafkaBrokers,
		RepositorySyncTopic:         sharedconfig.GetString("SYNC_SERVICE_REPOSITORY_SYNC_TOPIC", events.RepositorySyncRequestedTopic),
		RepositorySyncConsumerGroup: sharedconfig.GetString("SYNC_SERVICE_CONSUMER_GROUP", "codeatlas-sync-service"),
		GitHubAppSlug:               sharedconfig.GetString("GITHUB_APP_SLUG", ""),
		GitHubAppID:                 int64(githubAppID),
		GitHubAppClientID:           sharedconfig.GetString("GITHUB_APP_CLIENT_ID", ""),
		GitHubAppPrivateKeyPath:     sharedconfig.GetString("GITHUB_APP_PRIVATE_KEY_PATH", ""),
		GitHubAPIBaseURL:            sharedconfig.GetString("GITHUB_API_BASE_URL", "https://api.github.com"),
	}, nil
}

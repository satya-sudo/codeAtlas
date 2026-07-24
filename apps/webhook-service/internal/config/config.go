package config

import (
	"strings"
	"time"

	sharedconfig "codeatlas/packages/config"
	"codeatlas/packages/database"
	"codeatlas/packages/events"
	sharedkafka "codeatlas/packages/kafka"
)

type Config struct {
	ServiceName         string
	AppEnv              string
	LogLevel            string
	LogJSON             bool
	HTTPPort            int
	ShutdownTimeout     time.Duration
	GitHubWebhookSecret string
	KafkaEnabled        bool
	KafkaBrokers        []string
	GitHubPushTopic     string
	Postgres            database.PostgresConfig
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

	kafkaEnabled, err := sharedconfig.GetBool("WEBHOOK_SERVICE_KAFKA_ENABLED", false)
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

	postgresCfg, err := database.LoadPostgresConfigFromEnv()
	if err != nil {
		return Config{}, err
	}

	return Config{
		ServiceName:         "webhook-service",
		AppEnv:              sharedconfig.GetString("APP_ENV", "development"),
		LogLevel:            sharedconfig.GetString("LOG_LEVEL", "info"),
		LogJSON:             logJSON,
		HTTPPort:            httpPort,
		ShutdownTimeout:     shutdownTimeout,
		GitHubWebhookSecret: sharedconfig.GetString("GITHUB_WEBHOOK_SECRET", ""),
		KafkaEnabled:        kafkaEnabled,
		KafkaBrokers:        kafkaBrokers,
		GitHubPushTopic:     sharedconfig.GetString("WEBHOOK_SERVICE_GITHUB_PUSH_TOPIC", events.GitHubPushReceivedTopic),
		Postgres:            postgresCfg,
	}, nil
}

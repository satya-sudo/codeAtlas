package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	kafkago "github.com/segmentio/kafka-go"
)

type Producer interface {
	Publish(ctx context.Context, topic string, key string, payload any) error
	Close() error
}

type ProducerConfig struct {
	Logger   *slog.Logger
	Enabled  bool
	Brokers  []string
	ClientID string
}

type LoggingProducer struct {
	logger  *slog.Logger
	enabled bool
}

type BrokerProducer struct {
	writer *kafkago.Writer
}

func NewProducer(cfg ProducerConfig) Producer {
	if !cfg.Enabled || len(cfg.Brokers) == 0 {
		return &LoggingProducer{
			logger:  cfg.Logger,
			enabled: cfg.Enabled,
		}
	}

	return &BrokerProducer{
		writer: &kafkago.Writer{
			Addr:         kafkago.TCP(cfg.Brokers...),
			Balancer:     &kafkago.LeastBytes{},
			RequiredAcks: kafkago.RequireOne,
			Async:        false,
			Transport: &kafkago.Transport{
				ClientID: cfg.ClientID,
			},
		},
	}
}

func (p *LoggingProducer) Publish(_ context.Context, topic string, key string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	if p.enabled {
		p.logger.Info("kafka enabled but no brokers configured, skipping publish", "topic", topic, "key", key, "payload", string(body))
		return nil
	}

	p.logger.Info("kafka disabled, skipping publish", "topic", topic, "key", key, "payload", string(body))
	return nil
}

func (p *LoggingProducer) Close() error {
	return nil
}

func (p *BrokerProducer) Publish(ctx context.Context, topic string, key string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	return p.writer.WriteMessages(ctx, kafkago.Message{
		Topic: topic,
		Key:   []byte(key),
		Value: body,
		Time:  time.Now().UTC(),
	})
}

func (p *BrokerProducer) Close() error {
	return p.writer.Close()
}

func ParseBrokers(raw string) ([]string, error) {
	parts := strings.Split(raw, ",")
	brokers := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			continue
		}
		brokers = append(brokers, value)
	}

	if len(brokers) == 0 {
		return nil, fmt.Errorf("at least one kafka broker is required")
	}

	return brokers, nil
}

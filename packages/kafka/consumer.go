package kafka

import (
	"context"

	kafkago "github.com/segmentio/kafka-go"
)

type Message struct {
	Topic string
	Key   []byte
	Value []byte
}

type Consumer interface {
	FetchMessage(ctx context.Context) (Message, error)
	CommitMessages(ctx context.Context, messages ...Message) error
	Close() error
}

type ConsumerConfig struct {
	Brokers []string
	GroupID string
	Topic   string
}

type BrokerConsumer struct {
	reader *kafkago.Reader
}

func NewConsumer(cfg ConsumerConfig) Consumer {
	return &BrokerConsumer{
		reader: kafkago.NewReader(kafkago.ReaderConfig{
			Brokers: cfg.Brokers,
			GroupID: cfg.GroupID,
			Topic:   cfg.Topic,
		}),
	}
}

func (c *BrokerConsumer) FetchMessage(ctx context.Context) (Message, error) {
	msg, err := c.reader.FetchMessage(ctx)
	if err != nil {
		return Message{}, err
	}

	return Message{
		Topic: msg.Topic,
		Key:   msg.Key,
		Value: msg.Value,
	}, nil
}

func (c *BrokerConsumer) CommitMessages(ctx context.Context, messages ...Message) error {
	kafkaMessages := make([]kafkago.Message, 0, len(messages))
	for _, msg := range messages {
		kafkaMessages = append(kafkaMessages, kafkago.Message{
			Topic: msg.Topic,
			Key:   msg.Key,
			Value: msg.Value,
		})
	}

	return c.reader.CommitMessages(ctx, kafkaMessages...)
}

func (c *BrokerConsumer) Close() error {
	return c.reader.Close()
}

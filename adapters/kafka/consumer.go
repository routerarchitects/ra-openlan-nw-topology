package kafka

import (
	"context"
	"errors"
	"time"

	kgo "github.com/segmentio/kafka-go"

	"github.com/router-architects/ra-openlan-nw-topology/adapters/apperrors"
	"github.com/router-architects/ra-openlan-nw-topology/adapters/logger"
	"github.com/router-architects/ra-openlan-nw-topology/internal/config"
)

type Consumer struct {
	reader   *kgo.Reader
	registry *Registry
}

func NewConsumer(cfg *config.KafkaConfig, registry *Registry) (*Consumer, error) {
	if cfg == nil {
		return nil, apperrors.WrapError(apperrors.CodeInternal, "kafka: config is nil", nil)
	}
	if registry == nil {
		return nil, apperrors.WrapError(apperrors.CodeInternal, "kafka: registry is nil", nil)
	}

	topics := registry.Topics()
	if len(topics) == 0 {
		return nil, apperrors.WrapError(apperrors.CodeInternal, "kafka: no topics registered in registry", nil)
	}
	if len(cfg.KafkaBrokers) == 0 {
		return nil, apperrors.WrapError(apperrors.CodeInternal, "kafka: no brokers configured", nil)
	}

	dialTimeout := cfg.KafkaDialTimeout
	if dialTimeout <= 0 {
		dialTimeout = 5 * time.Second
	}

	maxWait := cfg.KafkaReadTimeout
	if maxWait <= 0 {
		maxWait = 1 * time.Second
	}

	dialer := &kgo.Dialer{
		Timeout:   dialTimeout,
		DualStack: true,
	}
	startOffset := kgo.LastOffset
	if cfg.KafkaAllowOffsetReset {
		startOffset = kgo.FirstOffset
	}

	reader := kgo.NewReader(kgo.ReaderConfig{
		Brokers:               cfg.KafkaBrokers,
		GroupID:               cfg.KafkaGroupID,
		GroupTopics:           topics, // use registry topics, not cfg.KafkaTopics
		Dialer:                dialer,
		MinBytes:              cfg.KafkaMinBytes,
		MaxBytes:              cfg.KafkaMaxBytes,
		CommitInterval:        0, // manual commit after successful publish to channel
		WatchPartitionChanges: true,
		ReadLagInterval:       -1,
		StartOffset:           startOffset,
		MaxWait:               maxWait,
		ReadBackoffMin:        250 * time.Millisecond,
		ReadBackoffMax:        2 * time.Second,
	})

	return &Consumer{
		reader:   reader,
		registry: registry,
	}, nil
}

func (c *Consumer) Run(ctx context.Context) error {
	log := logger.GetLoggerThreadId("KAFKA-CONSUMER")

	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if log != nil {
				log.WithFields(logger.Fields{"error": err}).Error("err in kafka consumer fetch message")
			}
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || ctx.Err() != nil {
				return nil
			}

			select {
			case <-time.After(500 * time.Millisecond):
			case <-ctx.Done():
				return nil
			}
			continue
		}

		ch, ok := c.registry.Channel(msg.Topic)
		if !ok {
			// No component registered for this topic; commit and move on.
			if log != nil {
				log.WithFields(logger.Fields{
					"component": "kafka.consumer",
					"topic":     msg.Topic,
				}).Warn("no channel registered; committing message")
			}
			_ = c.reader.CommitMessages(ctx, msg)
			continue
		}

		bmsg := Message{
			Topic: msg.Topic,
			Value: msg.Value,
		}

		select {
		case ch <- bmsg:
			// Successfully delivered to internal channel; commit offset.
			_ = c.reader.CommitMessages(ctx, msg)
		case <-ctx.Done():
			return nil
		}
	}
}

// Close stops the underlying reader.
func (c *Consumer) Close() error {
	if c == nil || c.reader == nil {
		return nil
	}
	return c.reader.Close()
}

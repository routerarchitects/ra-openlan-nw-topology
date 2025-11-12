package kafka

import (
	"context"
	"errors"
	"time"

	kgo "github.com/segmentio/kafka-go"

	"github.com/router-architects/network-topology-service/internal/apperrors"
	"github.com/router-architects/network-topology-service/internal/config"
	internalkafka "github.com/router-architects/network-topology-service/internal/kafka"
	"github.com/router-architects/network-topology-service/internal/logger"
)

var (
	ErrNoTopics  = errors.New("kafka: no topics registered")
	ErrNoBrokers = errors.New("kafka: no brokers configured")
)

type Consumer struct {
	reader          *kgo.Reader
	handlerRegistry *internalkafka.HandlerRegistry
}

func NewConsumer(cfg *config.Config, handlerRegistry *internalkafka.HandlerRegistry) (*Consumer, error) {
	if cfg == nil {
		return nil, apperrors.WrapError(apperrors.CodeInternal, "kafka: config is nil", nil)
	}
	if handlerRegistry == nil {
		return nil, apperrors.WrapError(apperrors.CodeInternal, "kafka: registry is nil", nil)
	}

	topics := handlerRegistry.Topics()
	if len(topics) == 0 {
		return nil, ErrNoTopics
	}
	if len(cfg.KafkaBrokers) == 0 {
		return nil, ErrNoBrokers
	}

	if log := logger.GetLogger(); log != nil {
		log.WithFields(logger.Fields{
			"component": "kafka.consumer",
			"brokers":   cfg.KafkaBrokers,
			"group_id":  cfg.KafkaGroupID,
			"topics":    topics,
		}).Info("initializing kafka consumer")
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

	reader := kgo.NewReader(kgo.ReaderConfig{
		Brokers:               cfg.KafkaBrokers,
		GroupID:               cfg.KafkaGroupID,
		GroupTopics:           cfg.KafkaTopics,
		Dialer:                dialer,
		MinBytes:              cfg.KafkaMinBytes,
		MaxBytes:              cfg.KafkaMaxBytes,
		CommitInterval:        0, // manual commit after handler success
		WatchPartitionChanges: true,
		ReadLagInterval:       -1,
		StartOffset:           kgo.FirstOffset,
		MaxWait:               maxWait,
		ReadBackoffMin:        250 * time.Millisecond,
		ReadBackoffMax:        2 * time.Second,
	})

	if log := logger.GetLogger(); log != nil {
		log.WithFields(logger.Fields{
			"component": "kafka.consumer",
			"group_id":  cfg.KafkaGroupID,
			"topics":    topics,
		}).Info("kafka consumer ready")
	}

	return &Consumer{
		reader:          reader,
		handlerRegistry: handlerRegistry,
	}, nil
}

func (c *Consumer) Run(ctx context.Context) error {
	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || ctx.Err() != nil {
				return nil
			}

			logger.GetLogger().WithField("component", "kafka.consumer").WithError(err).Error("fetch message failed")

			select {
			case <-time.After(500 * time.Millisecond):
			case <-ctx.Done():
				return nil
			}
			continue
		}

		handler, ok := c.handlerRegistry.HandlerForTopic(msg.Topic)
		if !ok {
			logger.GetLogger().WithFields(logger.Fields{
				"component": "kafka.consumer",
				"topic":     msg.Topic,
			}).Warn("no handler registered; committing message")
			if err := c.reader.CommitMessages(ctx, msg); err != nil {
				logger.GetLogger().WithField("component", "kafka.consumer").WithError(err).Error("commit failed for unhandled topic")
			}
			continue
		}

		if err := handler.Handle(ctx, msg); err != nil {
			logger.GetLogger().WithFields(logger.Fields{
				"component": "kafka.consumer",
				"topic":     msg.Topic,
			}).WithError(err).Error("handler failed")
			// do not commit so the message can be retried
			continue
		}

		if err := c.reader.CommitMessages(ctx, msg); err != nil {
			logger.GetLogger().WithFields(logger.Fields{
				"component": "kafka.consumer",
				"topic":     msg.Topic,
			}).WithError(err).Error("commit failed")
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

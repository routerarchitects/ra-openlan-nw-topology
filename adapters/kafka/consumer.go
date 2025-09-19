package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/router-architects/network-topology-service/internal/config"
	"github.com/router-architects/network-topology-service/internal/logger"
	"github.com/router-architects/network-topology-service/internal/models"

	kgo "github.com/segmentio/kafka-go"
)

type channelStore interface {
	Deliver(uuid string, payload models.KafkaResponse) bool
	IsEmpty() bool
}

type consumer struct {
	r     *kgo.Reader
	cfg   *config.Config
	store channelStore
}

func NewConsumer(cfg *config.Config, store channelStore) (*consumer, error) {
	r := kgo.NewReader(kgo.ReaderConfig{
		Brokers:     cfg.KafkaBrokers,
		GroupID:     fmt.Sprintf("%s_%d", cfg.KafkaGroupID, time.Now().Unix()), // New group Id everytime to get latest messages
		Topic:       cfg.KafkaTopicResp,
		MinBytes:    cfg.KafkaMinBytes,
		MaxBytes:    cfg.KafkaMaxBytes,
		Dialer:      &kgo.Dialer{Timeout: cfg.KafkaDialTimeout},
		StartOffset: kgo.LastOffset,
	})

	return &consumer{r: r, cfg: cfg, store: store}, nil
}

func (c *consumer) Run(ctx context.Context) {
	logger.GetLogger().WithFields(logger.Fields{
		"component": "kafka.consumer",
		"topic":     c.cfg.KafkaTopicResp,
	}).Info("kafka consumer started")

	for {
		m, err := c.r.FetchMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				logger.GetLogger().WithFields(logger.Fields{"component": "kafka.consumer"}).Warn("context canceled")
				return
			}
			logger.GetLogger().WithFields(logger.Fields{"component": "kafka.consumer"}).WithError(err).Error("fetch error")
			time.Sleep(time.Second)
			continue
		}

		if c.store.IsEmpty() {
			// No pending correlations to deliver; commit and skip to avoid backlog.
			_ = c.r.CommitMessages(ctx, m)
			continue
		}
		logger.GetLogger().Trace("Kafka Message: ", string(m.Value))
		var resp models.KafkaResponse
		if err := json.Unmarshal(m.Value, &resp); err != nil {
			logger.GetLogger().WithFields(logger.Fields{"component": "kafka.consumer", "key": string(m.Key)}).WithError(err).Error("invalid response JSON")
			_ = c.r.CommitMessages(ctx, m)
			continue
		}

		if delivered := c.store.Deliver(resp.UUID, resp); delivered {
			logger.GetLogger().WithFields(logger.Fields{"component": "kafka.consumer", "uuid": resp.UUID}).Debug("delivered correlated response")
		} else {
			logger.GetLogger().WithFields(logger.Fields{"component": "kafka.consumer", "uuid": resp.UUID}).Trace("orphan response (expired/unknown uuid)")
		}
		_ = c.r.CommitMessages(ctx, m)
	}
}

func (c *consumer) Close() error { return c.r.Close() }

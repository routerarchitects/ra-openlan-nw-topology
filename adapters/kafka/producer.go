package kafka

import (
	"context"
	"time"

	"github.com/router-architects/ra-openlan-nw-topology/adapters/logger"
	"github.com/router-architects/ra-openlan-nw-topology/internal/config"

	"github.com/segmentio/kafka-go"
	kgo "github.com/segmentio/kafka-go"
)

type producer struct {
	w   *kgo.Writer
	cfg *config.KafkaConfig
}

func NewProducerForTopic(cfg *config.KafkaConfig, topic string) (*producer, error) {

	tr := &kafka.Transport{
		DialTimeout: 5 * time.Second,
		IdleTimeout: 30 * time.Second,
	}

	w := &kgo.Writer{
		Addr:                   kgo.TCP(cfg.KafkaBrokers...),
		Topic:                  topic,
		Balancer:               &kgo.Hash{}, // ensures same key -> same partition
		Transport:              tr,
		BatchTimeout:           10 * time.Millisecond,
		RequiredAcks:           -1, // wait for all replicas; more reliable during bring-up
		Async:                  false,
		AllowAutoTopicCreation: cfg.KafkaAllowAutoCreate, // MaxAttempts helps during broker leader elections / restarts.
		MaxAttempts:            12,
	}

	return &producer{w: w, cfg: cfg}, nil
}

func (p *producer) Publish(ctx context.Context, key string, payload []byte) error {
	log := logger.GetLoggerThreadId("SERVER")
	msg := kgo.Message{
		Key:   []byte(key),
		Value: payload,
		Time:  time.Now(),
	}

	if log != nil {
		log.WithFields(logger.Fields{
			"component": "kafka.producer",
			"uuid":      key,
			"size":      len(payload),
		}).Tracef("kafka publish with payload: %s", string(payload))
	}

	err := p.w.WriteMessages(ctx, msg)
	if err != nil {
		log.WithFields(logger.Fields{"err": err, "uuid": key}).Error("Publish failed.", err)
	}
	return err
}

func (p *producer) Close() error { return p.w.Close() }

package discovery

import (
	"context"
	"encoding/json"

	"github.com/router-architects/ra-openlan-nw-topology/adapters/apperrors"
	"github.com/router-architects/ra-openlan-nw-topology/adapters/kafka"
	"github.com/router-architects/ra-openlan-nw-topology/adapters/logger"
	"github.com/router-architects/ra-openlan-nw-topology/internal/models"
)

// DiscoveryComponent consumes discovery events from a Kafka-backed channel
// and updates the DiscoveryStore.
type DiscoveryComponent struct {
	topic string
	ch    <-chan kafka.Message
	store *DiscoveryStore
}

func NewDiscoveryComponent(
	topic string,
	registry *kafka.Registry,
	discoveryStore *DiscoveryStore,
	bufSize int,
) (*DiscoveryComponent, error) {
	if topic == "" {
		return nil, apperrors.WrapError(apperrors.CodeInternal, "discovery: topic is required", nil)
	}
	if registry == nil {
		return nil, apperrors.WrapError(apperrors.CodeInternal, "discovery: registry is nil", nil)
	}
	if discoveryStore == nil {
		return nil, apperrors.WrapError(apperrors.CodeInternal, "discovery: store is nil", nil)
	}
	if bufSize <= 0 {
		bufSize = 100
	}

	ch := make(chan kafka.Message, bufSize)

	// Register topic -> channel mapping with the internal registry.
	if err := registry.Register(topic, ch); err != nil {
		return nil, apperrors.WrapError(apperrors.CodeInternal, "discovery: failed to register topic", err)
	}

	return &DiscoveryComponent{
		topic: topic,
		ch:    ch,
		store: discoveryStore,
	}, nil
}

// Run should typically be started as a goroutine.
// It continuously reads messages from the channel and updates the store.
func (d *DiscoveryComponent) Run(ctx context.Context) {
	log := logger.GetLoggerThreadId("DISCOVERY")

	for {
		select {
		case <-ctx.Done():
			return

		case msg := <-d.ch:
			var evt models.DiscoveryEvent
			if err := json.Unmarshal(msg.Value, &evt); err != nil {
				if log != nil {
					log.WithError(err).Error("decode discovery event")
				}
				continue
			}

			if evt.Type == "" {
				continue
			}

			d.store.Upsert(evt)

			if log != nil {
				log.WithFields(logger.Fields{
					"component": "service.discovery",
					"service":   evt.Type,
					"event":     evt.Event,
				}).Trace("discovery event processed")
			}
		}
	}
}

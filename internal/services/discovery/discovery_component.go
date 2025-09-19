package discovery

import (
	"context"
	"encoding/json"

	kgo "github.com/segmentio/kafka-go"

	"github.com/router-architects/network-topology-service/internal/apperrors"
	"github.com/router-architects/network-topology-service/internal/domain"
	kafkarouter "github.com/router-architects/network-topology-service/internal/kafka"
	"github.com/router-architects/network-topology-service/internal/logger"
	"github.com/router-architects/network-topology-service/internal/store"
)

type DiscoveryHandler struct {
	topic          string
	discoveryStore *store.DiscoveryStore
}

func NewDiscoveryHandler(topic string, discoveryStore *store.DiscoveryStore) (*DiscoveryHandler, error) {
	if topic == "" {
		return nil, apperrors.WrapError(apperrors.CodeInternal, "discovery component: topic is required", nil)
	}
	if discoveryStore == nil {
		return nil, apperrors.WrapError(apperrors.CodeInternal, "discovery component: store is nil", nil)
	}
	return &DiscoveryHandler{topic: topic, discoveryStore: discoveryStore}, nil
}

func (h *DiscoveryHandler) Topic() string { return h.topic }

func (h *DiscoveryHandler) Handle(ctx context.Context, msg kgo.Message) error {
	_ = ctx

	var evt domain.DiscoveryEvent
	if err := json.Unmarshal(msg.Value, &evt); err != nil {
		return apperrors.WrapError(apperrors.CodeInvalidInput, "decode discovery event", err)
	}

	if evt.Type == "" {
		return apperrors.WrapError(apperrors.CodeInvalidInput, "discovery event missing service type", nil)
	}

	h.discoveryStore.Upsert(evt)

	logger.GetLogger().WithFields(logger.Fields{
		"component": "service.discovery",
		"service":   evt.Type,
		"event":     evt.Event,
	}).Trace("discovery event processed")

	return nil
}

var _ kafkarouter.TopicHandler = (*DiscoveryHandler)(nil)

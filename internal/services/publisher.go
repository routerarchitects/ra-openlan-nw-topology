package services

import "context"

// Publisher defines the minimal contract for publishing lifecycle events.
// Any implementation (e.g., Kafka producer) that provides this method can be injected.
type Publisher interface {
    Publish(ctx context.Context, key string, payload []byte) error
}


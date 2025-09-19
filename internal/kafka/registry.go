package kafka

import (
	"context"
	"errors"
	"sort"
	"sync"

	kgo "github.com/segmentio/kafka-go"
)

// TopicHandler processes Kafka messages for a single topic.
type TopicHandler interface {
	Topic() string
	Handle(ctx context.Context, msg kgo.Message) error
}

var (
	ErrNilHandler     = errors.New("kafka: handler is nil")
	ErrEmptyTopic     = errors.New("kafka: handler topic is empty")
	ErrDuplicateTopic = errors.New("kafka: handler already registered for topic")
)

type HandlerRegistry struct {
	mu       sync.RWMutex
	handlers map[string]TopicHandler
}

func NewHandlerRegistry() *HandlerRegistry {
	return &HandlerRegistry{handlers: make(map[string]TopicHandler)}
}

func (r *HandlerRegistry) RegisterHandler(c TopicHandler) error {
	if c == nil {
		return ErrNilHandler
	}

	topic := c.Topic()
	if topic == "" {
		return ErrEmptyTopic
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.handlers[topic]; exists {
		return ErrDuplicateTopic
	}
	r.handlers[topic] = c
	return nil
}

func (r *HandlerRegistry) HandlerForTopic(topic string) (TopicHandler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	c, ok := r.handlers[topic]
	return c, ok
}

func (r *HandlerRegistry) Topics() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	topics := make([]string, 0, len(r.handlers))
	for topic := range r.handlers {
		topics = append(topics, topic)
	}
	sort.Strings(topics)
	return topics
}

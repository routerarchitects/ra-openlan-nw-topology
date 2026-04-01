package kafka

import (
	"sort"
	"sync"

	"github.com/router-architects/ra-openlan-nw-topology/adapters/apperrors"
)

type Message struct {
	Topic string
	Value []byte
}

type Registry struct {
	mu    sync.RWMutex
	chans map[string]chan<- Message
}

func NewRegistry() *Registry {
	return &Registry{
		chans: make(map[string]chan<- Message),
	}
}

func (r *Registry) Register(topic string, ch chan<- Message) error {
	if topic == "" {
		return apperrors.WrapError(apperrors.CodeInternal, "kafka-registry: topic cannot be empty", nil)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.chans[topic]; exists {
		return apperrors.WrapError(apperrors.CodeInternal, "kafka-registry: topic already registered", nil)
	}

	r.chans[topic] = ch
	return nil
}

func (r *Registry) Channel(topic string) (chan<- Message, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ch, exists := r.chans[topic]
	return ch, exists
}

func (r *Registry) Topics() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]string, 0, len(r.chans))
	for t := range r.chans {
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}

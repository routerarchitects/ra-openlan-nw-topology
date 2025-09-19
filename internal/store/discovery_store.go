package store

import (
	"sync"

	"github.com/router-architects/network-topology-service/internal/domain"
)

type DiscoveryStore struct {
	mu            sync.RWMutex
	byServiceType map[string]domain.DiscoveryEvent
}

func NewDiscoveryStore() *DiscoveryStore {
	return &DiscoveryStore{byServiceType: make(map[string]domain.DiscoveryEvent)}
}

func (s *DiscoveryStore) Upsert(evt domain.DiscoveryEvent) {
	if evt.Type == "" {
		return
	}

	s.mu.Lock()
	s.byServiceType[evt.Type] = evt
	s.mu.Unlock()
}

func (s *DiscoveryStore) Get(serviceType string) (domain.DiscoveryEvent, bool) {
	s.mu.RLock()
	evt, ok := s.byServiceType[serviceType]
	s.mu.RUnlock()
	return evt, ok
}

func (s *DiscoveryStore) Snapshot() map[string]domain.DiscoveryEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make(map[string]domain.DiscoveryEvent, len(s.byServiceType))
	for k, v := range s.byServiceType {
		out[k] = v
	}
	return out
}

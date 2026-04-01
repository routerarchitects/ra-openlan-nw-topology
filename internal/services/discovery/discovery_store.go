package discovery

import (
	"strings"
	"sync"

	"github.com/router-architects/ra-openlan-nw-topology/internal/models"
)

type DiscoveryStore struct {
	mu                sync.RWMutex
	byPrivateEndpoint map[string]models.DiscoveryEvent
}

func NewDiscoveryStore() *DiscoveryStore {
	return &DiscoveryStore{byPrivateEndpoint: make(map[string]models.DiscoveryEvent)}
}

func (s *DiscoveryStore) Upsert(evt models.DiscoveryEvent) {
	privateEndpoint := strings.TrimSpace(evt.PrivateEndPoint)
	svcType := strings.TrimSpace(evt.Type)
	if privateEndpoint == "" || svcType == "" {
		return
	}

	eventName := strings.ToLower(strings.TrimSpace(evt.Event))

	s.mu.Lock()
	switch eventName {
	case "keep-alive", "join":
		s.byPrivateEndpoint[privateEndpoint] = evt
	default:
		delete(s.byPrivateEndpoint, privateEndpoint)
	}
	s.mu.Unlock()
}

func (s *DiscoveryStore) GetServices(serviceType string) []models.DiscoveryEvent {
	serviceType = strings.TrimSpace(serviceType)
	if serviceType == "" {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]models.DiscoveryEvent, 0, len(s.byPrivateEndpoint))
	for _, evt := range s.byPrivateEndpoint {
		if strings.EqualFold(evt.Type, serviceType) {
			out = append(out, evt)
		}
	}
	return out
}

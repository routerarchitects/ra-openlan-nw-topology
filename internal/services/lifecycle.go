package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math/rand/v2"
	"time"

	"github.com/router-architects/network-topology-service/internal/config"
	"github.com/router-architects/network-topology-service/internal/logger"
	// "github.com/router-architects/cgw-wrapper/internal/observability/logx"
)

type lifecycleService struct {
	cfg      *config.Config
	pub      Publisher
	instance lifecycleState
}

type lifecycleState struct {
	id      int64
	key     string
	stype   string // service type
	version string
	privURI string
	pubURI  string
}

type lifecycleEvent struct {
	Event           string `json:"event"`
	ID              int64  `json:"id"`
	Key             string `json:"key"`
	PrivateEndPoint string `json:"privateEndPoint"`
	PublicEndPoint  string `json:"publicEndPoint"`
	Type            string `json:"type"`
	Version         string `json:"version"`
}

func NewLifecycleService(cfg *config.Config, pub Publisher) *lifecycleService {
	// build stable state for this process
	id := uniqueNanoID()
	key := sha256Hex(cfg.PublicEndpoint)
	return &lifecycleService{
		cfg: cfg,
		pub: pub,
		instance: lifecycleState{
			id:      id,
			key:     key,
			stype:   cfg.ServiceType,
			version: cfg.BuildVersion,
			privURI: cfg.PrivateEndpoint,
			pubURI:  cfg.PublicEndpoint,
		},
	}
}

// Start sends "join" once, then "keep-alive" at interval; on ctx cancel sends "leave" once.
func (s *lifecycleService) Start(ctx context.Context) {
	// immediate join
	s.safePublish(ctx, "join")

	// ticker for keep-alive
	ival := s.cfg.LifecycleInterval
	if ival <= 0 {
		ival = 30 * time.Second
	}
	t := time.NewTicker(ival)
	go func() {
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				s.safePublish(context.Background(), "leave")
				return
			case <-t.C:
				s.safePublish(ctx, "keep-alive")
			}
		}
	}()
}

func (s *lifecycleService) safePublish(ctx context.Context, ev string) {
	evt := lifecycleEvent{
		Event:           ev,
		ID:              s.instance.id,
		Key:             s.instance.key,
		PrivateEndPoint: s.instance.privURI,
		PublicEndPoint:  s.instance.pubURI,
		Type:            s.instance.stype,
		Version:         s.instance.version,
	}
	b, err := json.Marshal(evt)
	if err != nil {
		logger.GetLogger().WithError(err).Error("lifecycle: marshal failed")
		return
	}

	// use the hash key to keep events for this instance in a stable partition
	if err := s.pub.Publish(ctx, s.instance.privURI, b); err != nil {
		logger.GetLogger().WithFields(logger.Fields{
			"component": "lifecycle",
			"event":     ev,
		}).WithError(err).Error("lifecycle publish failed")
		return
	}
	logger.GetLogger().WithFields(logger.Fields{
		"component": "lifecycle",
		"event":     ev,
	}).Trace("lifecycle event published")
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// uniqueNanoID returns a 63-bit positive int composed of current unix-nano with a few random low bits.
// This preserves time ordering while adding per-process randomness.
func uniqueNanoID() int64 {
	n := time.Now().UnixNano() & 0x7fffffffffffffff
	// low 12 random bits to reduce collision risk across instances
	r := int64(rand.Uint32() & 0x0fff)
	return (n &^ 0x0fff) | r
}

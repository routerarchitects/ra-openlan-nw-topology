package logger

import (
	"sync"

	"github.com/sirupsen/logrus"
)

var (
	mu              sync.RWMutex
	subsystemLevels = make(map[string]logrus.Level)
	// keep default known subsystem names in sync with logger.threadName constants
	defaultSubsystemNames = []string{
		ThreadKafkaConsumer,
		ThreadDiscovery,
		ThreadServer,
	}
)

type SubsystemLogLevel struct {
	Tag   string `json:"tag"`
	Value string `json:"value"`
}

// Set dynamic loglevel for subsystem
func SetSubsystemLevel(name string, level logrus.Level) {
	mu.Lock()
	defer mu.Unlock()
	subsystemLevels[name] = level
}

// Return current level for subsystem (default: INFO)
func GetSubsystemLevel(name string) logrus.Level {
	mu.RLock()
	defer mu.RUnlock()

	if lvl, ok := subsystemLevels[name]; ok {
		return lvl
	}
	return logrus.InfoLevel
}

// Used by GETLOGLEVELS API
func ListSubsystemLogLevelPairs() []SubsystemLogLevel {
	mu.RLock()
	defer mu.RUnlock()

	out := make([]SubsystemLogLevel, 0, len(subsystemLevels))
	for name, lvl := range subsystemLevels {
		out = append(out, SubsystemLogLevel{
			Tag:   name,
			Value: lvl.String(),
		})
	}
	return out
}

// Used by GETSUBSYSTEMNAMES API
func ListSubsystemNames() []string {
	mu.RLock()
	defer mu.RUnlock()

	names := make([]string, 0, len(subsystemLevels))
	for name := range subsystemLevels {
		names = append(names, name)
	}
	return names
}

// InitializeSubsystemLevels stores the specified level for every default subsystem.
func InitializeSubsystemLevels(level string) {
	parsed, err := logrus.ParseLevel(level)
	if err != nil {
		parsed = logrus.InfoLevel
	}
	for _, name := range defaultSubsystemNames {
		SetSubsystemLevel(name, parsed)
	}
}

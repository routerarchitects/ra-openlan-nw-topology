package config

import (
	"github.com/caarlos0/env/v11"
	Subsystem "github.com/routerarchitects/ow-common-mods/fiber/system-routes"
	"github.com/routerarchitects/ow-common-mods/servicediscovery"
	kafka "github.com/routerarchitects/ra-common-mods/kafka"
	logger "github.com/routerarchitects/ra-common-mods/logger"
)

type ServerConfig struct {
	// server
	HTTPPort    int    `env:"HTTP_PORT" envDefault:"8088"`
	PrivatePort int    `env:"PRIVATE_HTTP_PORT" envDefault:"17007"`
	TLS_CERT    string `env:"INTERNAL_RESTAPI_HOST_CERT"`
	TLS_KEY     string `env:"INTERNAL_RESTAPI_HOST_KEY"`
	TLS_ROOTCA  string `env:"INTERNAL_RESTAPI_HOST_ROOTCA"`
}

type KafkaConfig struct {
	kafka.Config
}

type DiscoveryConfig struct {
	servicediscovery.Config
}

type LoggerConfig struct {
	logger.Config
}

type SubsystemConfig struct {
	Subsystem.Config
}

// HistoryConfig defines bounded resource limits for historical topology queries.
type HistoryConfig struct {
	MaxDurationSeconds      uint64 `env:"TOPOLOGY_HISTORY_MAX_DURATION_SECONDS" envDefault:"86400"`
	MaxJitterSeconds        uint64 `env:"TOPOLOGY_HISTORY_MAX_JITTER_SECONDS" envDefault:"300"`
	MaxSnapshotCount        uint64 `env:"TOPOLOGY_HISTORY_MAX_SNAPSHOT_COUNT" envDefault:"720"`
	MaxTimepointsPerFetch   int    `env:"TOPOLOGY_HISTORY_MAX_TIMEPOINTS_PER_FETCH" envDefault:"5000"`
	MaxTotalTimepoints      int    `env:"TOPOLOGY_HISTORY_MAX_TOTAL_TIMEPOINTS" envDefault:"100000"`
	MaxEstimatedOutputUnits uint64 `env:"TOPOLOGY_HISTORY_MAX_ESTIMATED_OUTPUT_UNITS" envDefault:"250000"`
}

func DefaultHistoryConfig() HistoryConfig {
	return HistoryConfig{
		MaxDurationSeconds:      86400,
		MaxJitterSeconds:        300,
		MaxSnapshotCount:        720,
		MaxTimepointsPerFetch:   5000,
		MaxTotalTimepoints:      100000,
		MaxEstimatedOutputUnits: 250000,
	}
}

type Config struct {
	Server    ServerConfig
	Kafka     KafkaConfig
	Discovery DiscoveryConfig
	Logger    LoggerConfig
	Subsystem SubsystemConfig
	History   HistoryConfig
}

func Load() (*Config, error) {
	var c Config
	if err := env.Parse(&c); err != nil {
		return nil, err
	}
	return &c, nil
}

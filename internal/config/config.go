package config

import (
	"github.com/caarlos0/env/v11"
	"github.com/router-architects/ra-openlan-nw-topology/internal/utils"
	servicediscovery "github.com/routerarchitects/ow-common-mods/servicediscovery"
	Subsystem "github.com/routerarchitects/ow-common-mods/system-routes"
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

type Config struct {
	Server    ServerConfig
	Kafka     KafkaConfig
	Discovery DiscoveryConfig
	Logger    LoggerConfig
	Subsystem SubsystemConfig
}

func Load() (*Config, error) {
	var c Config
	if err := env.Parse(&c); err != nil {
		return nil, err
	}
	return &c, nil
}

func (c LoggerConfig) ModuleConfig() logger.Config {
	return c.Config
}

func (c DiscoveryConfig) ModuleConfig() servicediscovery.Config {
	cfg := c.Config
	if cfg.InstanceID == 0 {
		cfg.InstanceID = utils.UniqueNanoID()
	}
	if cfg.InstanceKey == "" {
		keySource := cfg.PublicEndpoint
		cfg.InstanceKey = utils.Sha256Hex(keySource)
	}
	return cfg
}

func (c KafkaConfig) ModuleConfig() kafka.Config {
	return c.Config
}

func (c SubsystemConfig) ModuleConfig() Subsystem.Config {
	return c.Config
}

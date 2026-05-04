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

package config

import (
	"crypto/sha256"
	"encoding/hex"
	"math/rand/v2"
	"time"

	"github.com/caarlos0/env/v11"
	servicediscovery "github.com/routerarchitects/ow-common-mods/servicediscovery"
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

type Config struct {
	Server    ServerConfig
	Kafka     KafkaConfig
	Discovery DiscoveryConfig
	Logger    LoggerConfig
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
		cfg.InstanceID = uniqueNanoID()
	}
	if cfg.InstanceKey == "" {
		keySource := cfg.PublicEndpoint
		cfg.InstanceKey = sha256Hex(keySource)
	}
	return cfg
}

func (c KafkaConfig) ModuleConfig() kafka.Config {
	return c.Config
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func uniqueNanoID() int64 {
	n := time.Now().UnixNano() & 0x7fffffffffffffff
	// low 12 random bits to reduce collision risk across instances
	r := int64(rand.Uint32() & 0x0fff)
	return (n &^ 0x0fff) | r
}

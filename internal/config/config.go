package config

import (
	"time"

	"github.com/caarlos0/env/v11"
)

type ServerConfig struct {
	// server
	HTTPPort              int    `env:"HTTP_PORT" envDefault:"8088"`
	PrivatePort           int    `env:"PRIVATE_HTTP_PORT" envDefault:"17007"`
	TLS_CERT              string `env:"INTERNAL_RESTAPI_HOST_CERT"`
	TLS_KEY               string `env:"INTERNAL_RESTAPI_HOST_KEY"`
	TokenValidationCACert string `env:"INTERNAL_RESTAPI_HOST_ROOTCA"`
}

type KafkaConfig struct {
	// kafka
	KafkaBrokers          []string      `env:"KAFKA_BROKERS" envSeparator:","`
	KafkaTopicCmd         string        `env:"KAFKA_TOPIC_CMD" envDefault:"service_event"`
	KafkaGroupID          string        `env:"KAFKA_GROUP_ID" envDefault:"nwtopology-service-group"`
	KafkaDialTimeout      time.Duration `env:"KAFKA_DIAL_TIMEOUT" envDefault:"5s"`
	KafkaReadTimeout      time.Duration `env:"KAFKA_READ_TIMEOUT" envDefault:"5s"`
	KafkaMinBytes         int           `env:"KAFKA_MIN_BYTES" envDefault:"1"`
	KafkaMaxBytes         int           `env:"KAFKA_MAX_BYTES" envDefault:"1048576"`
	KafkaAllowAutoCreate  bool          `env:"KAFKA_ALLOW_AUTO_CREATE" envDefault:"true"`
	KafkaAllowOffsetReset bool          `env:"KAFKA_OFFSET_RESET" envDefault:"false"`
	KafkaTopicLifecycle   string        `env:"KAFKA_TOPIC_LIFECYCLE" envDefault:"service_events"`
}

type LifecycleConfig struct {
	// lifecycle event config
	PrivateEndpoint   string        `env:"SYSTEM_URI_PRIVATE"`
	PublicEndpoint    string        `env:"SYSTEM_URI_PUBLIC"`
	ServiceType       string        `env:"SERVICE_TYPE" envDefault:"nwtopology"`
	LifecycleInterval time.Duration `env:"LIFECYCLE_INTERVAL" envDefault:"30s"`
	BuildVersion      string        `env:"BUILD_VERSION" envDefault:"dev"`
}

type LoggerConfig struct {
	// logging
	LogLevel string `env:"SYSTEM_LOG_LEVEL" envDefault:"trace"`
}

type Config struct {
	Server    ServerConfig
	Kafka     KafkaConfig
	Lifecycle LifecycleConfig
	Logger    LoggerConfig
}

func Load() (*Config, error) {
	var c Config
	if err := env.Parse(&c); err != nil {
		return nil, err
	}
	return &c, nil
}

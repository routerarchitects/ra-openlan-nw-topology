package config

import (
	"log/slog"
	"time"

	"github.com/caarlos0/env/v11"
	servicediscovery "github.com/routerarchitects/ow-common-mods/servicediscovery"
	kafka "github.com/routerarchitects/ra-common-mods/kafka"
	logger "github.com/routerarchitects/ra-common-mods/logger"
)

type OrderingStrategy string

const (
	OrderingRoundRobin    OrderingStrategy = "round-robin"
	OrderingLastSeen      OrderingStrategy = "last-seen"
	OrderingLatestVersion OrderingStrategy = "latest-version"
	OrderingNone          OrderingStrategy = "none"
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
	// kafka
	KafkaBrokers         []string      `env:"KAFKA_BROKERS" envSeparator:","`
	KafkaClientID        string        `env:"KAFKA_CLIENT_ID" envDefault:"nwtopology-service"`
	KafkaTopicCmd        string        `env:"KAFKA_TOPIC_CMD" envDefault:"service_event"`
	KafkaGroupID         string        `env:"KAFKA_GROUP_ID" envDefault:"nwtopology-service-group"`
	KafkaInitialOffset   string        `env:"KAFKA_INITIAL_OFFSET" envDefault:"newest"`
	KafkaSessionTimeout  time.Duration `env:"KAFKA_SESSION_TIMEOUT" envDefault:"10s"`
	HeartbeatInterval    time.Duration `env:"KAFKA_HEARTBEAT_INTERVAL" envDefault:"1s"`
	KafkaMaxRetries      int           `env:"KAFKA_MAX_RETRIES" envDefault:"3"`
	KafkaProducerTimeout time.Duration `env:"KAFKA_PRODUCER_TIMEOUT" envDefault:"5s"`
	KafkaRequiredAcks    int           `env:"KAFKA_REQUIRED_ACKS" envDefault:"-1"`
	KafkaIndempotent     bool          `env:"KAFKA_INDEMPOTENT" envDefault:"true"`
}

type Servicediscovery struct {
	// Topic is the Kafka topic name used for broadcasting and consuming service discovery messages.
	Topic string `env:"DISCOVERY_TOPIC,required"`

	// ServiceType is the name of the service (e.g., "auth-service", "payment-service").
	ServiceType string `env:"DISCOVERY_SERVICE_TYPE,required"`
	// ServiceVersion indicates the version of the service deployment (e.g., "v1.0.0").
	ServiceVersion string `env:"DISCOVERY_SERVICE_VERSION,required"`
	// PrivateEndpoint is the internal network address (host:port) where the service listens.
	PrivateEndpoint string `env:"DISCOVERY_PRIVATE_ENDPOINT,required"`
	// PublicEndpoint is the external network address (host:port) accessible to clients, if applicable.
	PublicEndpoint string `env:"DISCOVERY_PUBLIC_ENDPOINT,required"`

	// InstanceID is an optional stable unique identifier for the instance.
	// If not provided, a random ID is generated at startup.
	InstanceID int64 `env:"DISCOVERY_INSTANCE_ID"`
	// InstanceKey is an optional stable key string for the instance.
	// If not provided, a random key is generated at startup.
	InstanceKey string `env:"DISCOVERY_INSTANCE_KEY"`

	// KeepAliveInterval is the duration between sending heartbeat messages to the discovery topic.
	KeepAliveInterval time.Duration `env:"DISCOVERY_KEEPALIVE_INTERVAL" envDefault:"5s"`
	// ExpiryMultiplier determines the timeout for considering an instance offline.
	// Timeout = KeepAliveInterval * ExpiryMultiplier.
	ExpiryMultiplier int `env:"DISCOVERY_EXPIRY_MULTIPLIER" envDefault:"2"`
	// SweepInterval is the frequency at which the local registry removes expired instances.
	SweepInterval time.Duration `env:"DISCOVERY_SWEEP_INTERVAL" envDefault:"10s"`

	// Ordering defines the strategy for sorting service instances when identifying the "best" instance.
	Ordering OrderingStrategy `env:"DISCOVERY_ORDERING" envDefault:"last-seen"`
}

type LoggerConfig struct {
	ServiceName    string `env:"SERVICE_NAME,required"`
	ServiceVersion string `env:"SERVICE_VERSION,,required"`
	Environment    string `env:"ENVIRONMENT" envDefault:"dev"`

	Output OutputConfig `envPrefix:"LOG_"`

	Levels LevelsConfig `envPrefix:"LOG_"`

	Redaction RedactionConfig `envPrefix:"LOG_REDACT_"`

	Stacktrace StacktraceConfig `envPrefix:"LOG_STACK_"`
}

type OutputConfig struct {
	// Format: "json" or "text"
	// If empty, defaults to:
	// - dev: text
	// - otherwise: json
	Format string `env:"FORMAT" envDefault:""`

	// AddSource: include file:line
	AddSource bool `env:"ADD_SOURCE" envDefault:"false"`
}

type LevelsConfig struct {
	// DefaultLevel applies when subsystem is not in SubsystemLevels.
	DefaultLevel string `env:"LEVEL" envDefault:"info"`

	// SubsystemLevelsRaw is the env-friendly encoding of subsystem levels.
	// Example: "http=info,db=warn,worker=debug"
	SubsystemLevelsRaw string `env:"SUBSYSTEM_LEVELS" envDefault:""`

	// SubsystemLevels is the parsed map (owned by module at runtime).
	// Services may leave this empty and rely on SubsystemLevelsRaw.
	SubsystemLevels map[string]slog.Level `env:"-"`
}

type RedactionConfig struct {
	Enabled bool `env:"ENABLED" envDefault:"false"`

	// KeysCSV is a denylist of keys whose values must be replaced.
	// If empty, module uses defaults internally.
	KeysCSV string `env:"KEYS" envDefault:"authorization,cookie,set-cookie,password,passwd,token,access_token,refresh_token,secret,api_key,x-api-key"`

	Replacement string `env:"REPLACEMENT" envDefault:"******"`
}

type StacktraceConfig struct {
	Enabled bool `env:"ENABLED" envDefault:"false"`

	// Level: include stacktrace for logs at/above this level (e.g. "error")
	Level string `env:"LEVEL" envDefault:"error"`
}

type Config struct {
	Server    ServerConfig
	Kafka     KafkaConfig
	Discovery Servicediscovery
	Logger    LoggerConfig
}

func Load() (*Config, error) {
	var c Config
	if err := env.Parse(&c); err != nil {
		return nil, err
	}
	return &c, nil
}

func GetLoggerConfig(cfg Config) logger.Config {
	return logger.Config{
		ServiceName:    cfg.Logger.ServiceName,
		ServiceVersion: cfg.Logger.ServiceVersion,
		Environment:    cfg.Logger.Environment,
		Levels: logger.LevelsConfig{
			DefaultLevel: cfg.Logger.Levels.DefaultLevel,
		},
		Output: logger.OutputConfig{
			Format: cfg.Logger.Output.Format,
		},
	}
}

func GetDiscoveryConfig(cfg Config) servicediscovery.Config {
	return servicediscovery.Config{
		Topic:             cfg.Discovery.Topic,
		ServiceType:       cfg.Discovery.ServiceType,
		ServiceVersion:    cfg.Discovery.ServiceVersion,
		PrivateEndpoint:   cfg.Discovery.PrivateEndpoint,
		PublicEndpoint:    cfg.Discovery.PublicEndpoint,
		InstanceID:        cfg.Discovery.InstanceID,
		InstanceKey:       cfg.Discovery.InstanceKey,
		KeepAliveInterval: cfg.Discovery.KeepAliveInterval,
		ExpiryMultiplier:  cfg.Discovery.ExpiryMultiplier,
		SweepInterval:     cfg.Discovery.SweepInterval,
		Ordering:          servicediscovery.OrderingStrategy(cfg.Discovery.Ordering),
	}

}

func GetKafkaConfig(cfg Config) kafka.Config {
	return kafka.Config{
		Brokers:  cfg.Kafka.KafkaBrokers,
		ClientID: cfg.Kafka.KafkaClientID,
		Consumer: kafka.ConsumerConfig{
			GroupID:           cfg.Kafka.KafkaGroupID,
			InitialOffset:     cfg.Kafka.KafkaInitialOffset,
			SessionTimeout:    cfg.Kafka.KafkaSessionTimeout,
			HeartbeatInterval: cfg.Kafka.HeartbeatInterval,
		},
		Producer: kafka.ProducerConfig{
			RequiredAcks: int16(cfg.Kafka.KafkaRequiredAcks),
			Idempotent:   cfg.Kafka.KafkaIndempotent,
			MaxRetries:   cfg.Kafka.KafkaMaxRetries,
			Timeout:      cfg.Kafka.KafkaProducerTimeout,
		},
	}
}

package main

import (
	"context"
	"crypto/x509"
	"os"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/client"
	"github.com/sirupsen/logrus"

	kafkaadapter "github.com/router-architects/network-topology-service/adapters/kafka"
	"github.com/router-architects/network-topology-service/adapters/postgres"
	"github.com/router-architects/network-topology-service/internal/config"
	"github.com/router-architects/network-topology-service/internal/http"
	"github.com/router-architects/network-topology-service/internal/http/handlers"
	"github.com/router-architects/network-topology-service/internal/kafka"
	"github.com/router-architects/network-topology-service/internal/logger"
	"github.com/router-architects/network-topology-service/internal/repositories"
	"github.com/router-architects/network-topology-service/internal/security"
	"github.com/router-architects/network-topology-service/internal/services"
	discoverycomponent "github.com/router-architects/network-topology-service/internal/services/discovery"
	"github.com/router-architects/network-topology-service/internal/store"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}

	log := logrus.New()
	log.SetOutput(os.Stdout)

	level, _ := logrus.ParseLevel(cfg.LogLevel)
	log.SetLevel(level)
	log.SetFormatter(&logrus.JSONFormatter{TimestampFormat: time.RFC3339Nano})
	entry := logrus.NewEntry(log)
	logger.SetLogger(logger.LogrusAdapter{entry})
	logger.SetLogger(logger.GetLogger().WithField("app", cfg.AppName))

	// pgx pool
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := postgres.NewPool(ctx, postgres.Config{
		Host:            cfg.PGHost,
		Port:            cfg.PGPort,
		User:            cfg.PGUser,
		Password:        cfg.PGPassword,
		Database:        cfg.PGDatabase,
		SSLMode:         cfg.PGSSLMode,
		MaxConns:        cfg.PGMaxConns,
		MinConns:        cfg.PGMinConns,
		MaxConnLifetime: cfg.PGMaxLifetime,
	})
	if err != nil {
		logger.GetLogger().WithError(err).Fatal("failed to connect postgres")
	}

	svcDiscoveryStore := store.NewDiscoveryStore()

	tokenValidationClient := client.New()
	tokenValidationClient.SetTimeout(5 * time.Second)
	if cfg.TokenValidationCACert != "" {
		pemBytes, err := os.ReadFile(cfg.TokenValidationCACert)
		if err != nil {
			logger.GetLogger().WithError(err).Fatal("failed to read token validation CA cert")
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pemBytes) {
			logger.GetLogger().Fatal("failed to parse token validation CA cert")
		}
		tokenValidationClient.TLSConfig().RootCAs = pool
	}

	tokenValidator := security.NewTokenValidator(
		svcDiscoveryStore,
		tokenValidationClient,
		security.ValidatorConfig{
			Timeout: 3 * time.Second,
		},
	)

	cmdProducer, err := kafkaadapter.NewProducerForTopic(cfg, cfg.KafkaTopicCmd)
	if err != nil {
		logger.GetLogger().WithError(err).Fatal("failed to init kafka producer")
	}

	lifecycleProducer, err := kafkaadapter.NewProducerForTopic(cfg, cfg.KafkaTopicLifecycle) // for lifecycle events
	if err != nil {
		logger.GetLogger().WithError(err).Fatal("failed to init kafka producer (lifecycle)")
	}

	handlerRegistry := kafka.NewHandlerRegistry()
	discoveryHandler, err := discoverycomponent.NewDiscoveryHandler(cfg.KafkaTopicLifecycle, svcDiscoveryStore)
	if err != nil {
		logger.GetLogger().WithError(err).Fatal("failed to create discovery component")
	}
	if err := handlerRegistry.RegisterHandler(discoveryHandler); err != nil {
		logger.GetLogger().WithError(err).Fatal("failed to register discovery component")
	}

	consumer, err := kafkaadapter.NewConsumer(cfg, handlerRegistry)
	if err != nil {
		logger.GetLogger().WithError(err).Info("failed to init kafka consumer")
	}

	lifecycleService := services.NewLifecycleService(cfg, lifecycleProducer)

	// wire
	repo := repositories.NewTopologyRepository(pool)
	svc := services.NewTopologyService(repo)

	app := fiber.New(fiber.Config{
		// optional: tune body limits, read/write timeouts are handled by env values for HTTP server if you run behind a reverse proxy
		ReadTimeout:  time.Second * 10,
		WriteTimeout: time.Second * 15,
	})

	th := handlers.NewTopologyHandler(svc)
	deps := http.ServerDeps{
		APIKey:         cfg.APIKey,
		TopologyWindow: cfg.TopologyWindow,
		TopologyDrift:  cfg.TopologyDrift,
		TokenValidator: tokenValidator,
	}

	http.New(app, deps, th)
	deps.RegisterRoutes(app, th)

	runCtx, runCancel := context.WithCancel(context.Background())
	defer runCancel()
	if consumer != nil {
		go func() {
			if err := consumer.Run(runCtx); err != nil {
				logger.GetLogger().WithError(err).Error("kafka consumer stopped")
			}
		}()
	}
	lifecycleService.Start(runCtx)

	err = (&deps).Start(app, *cfg, *pool)
	if err != nil {
		logger.GetLogger().WithError(err).Fatal("failed to start http server")
	}

	runCancel()
	_ = app.Shutdown()
	_ = cmdProducer.Close()
	if consumer != nil {
		_ = consumer.Close()
	}
}

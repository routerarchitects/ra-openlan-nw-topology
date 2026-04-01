package main

import (
	"context"
	"crypto/x509"
	"os"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/client"

	serviceclient "github.com/router-architects/ra-openlan-nw-topology/adapters/httpclient"
	kafkaadapter "github.com/router-architects/ra-openlan-nw-topology/adapters/kafka"
	"github.com/router-architects/ra-openlan-nw-topology/adapters/logger"
	"github.com/router-architects/ra-openlan-nw-topology/internal/api"
	"github.com/router-architects/ra-openlan-nw-topology/internal/api/handlers"
	"github.com/router-architects/ra-openlan-nw-topology/internal/api/middlewares"
	"github.com/router-architects/ra-openlan-nw-topology/internal/config"
	"github.com/router-architects/ra-openlan-nw-topology/internal/gateway/analytics"
	"github.com/router-architects/ra-openlan-nw-topology/internal/gateway/security"
	"github.com/router-architects/ra-openlan-nw-topology/internal/services"
	"github.com/router-architects/ra-openlan-nw-topology/internal/services/discovery"
	discoverycomponent "github.com/router-architects/ra-openlan-nw-topology/internal/services/discovery"
	"github.com/router-architects/ra-openlan-nw-topology/internal/services/lifecycle"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}

	logger.SetLogger(logger.NewLogrusLogger(cfg.Logger.LogLevel))
	logger.InitializeSubsystemLevels(cfg.Logger.LogLevel)
	log := logger.GetLogger()

	if log == nil {
		panic("failed to init logger")
	}

	log.Info("successfully init log level")

	svcDiscoveryStore := discovery.NewDiscoveryStore()

	fiberClient := client.New()
	fiberClient.SetTimeout(5 * time.Second)

	if cfg.Server.TokenValidationCACert != "" {
		pemBytes, err := os.ReadFile(cfg.Server.TokenValidationCACert)
		if err != nil {
			log.WithError(err).Fatal("failed to read token validation CA cert")
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pemBytes) {
			log.Fatal("failed to parse token validation CA cert")
		}
		fiberClient.TLSConfig().RootCAs = pool
	}

	OpenAPIRequestClient := serviceclient.NewOpenApiRequest(
		fiberClient,
		serviceclient.OpenAPIRequestConfig{
			Timeout: 3 * time.Second,
		},
	)

	tokenValidator := security.NewTokenValidator(
		OpenAPIRequestClient,
		svcDiscoveryStore,
	)
	analyticsClient := analytics.NewAnalyticsClient(OpenAPIRequestClient, svcDiscoveryStore)

	lifecycleProducer, err := kafkaadapter.NewProducerForTopic(&cfg.Kafka, cfg.Kafka.KafkaTopicLifecycle)
	if err != nil {
		log.WithError(err).Fatal("failed to init kafka producer (lifecycle)")
	}

	handlerRegistry := kafkaadapter.NewRegistry()
	discoveryComponent, err := discoverycomponent.NewDiscoveryComponent(cfg.Kafka.KafkaTopicCmd, handlerRegistry, svcDiscoveryStore, 100)
	if err != nil {
		log.WithError(err).Fatal("failed to create discovery component")
	}

	consumer, err := kafkaadapter.NewConsumer(&cfg.Kafka, handlerRegistry)
	if err != nil {
		log.WithError(err).Warn("failed to init kafka consumer")
	}

	lifecycleService := lifecycle.NewLifecycleService(&cfg.Lifecycle, lifecycleProducer)

	svc := services.NewTopologyService(analyticsClient)

	app := fiber.New(fiber.Config{
		ReadTimeout:  time.Second * 10,
		WriteTimeout: time.Second * 15,
	})

	th := handlers.NewTopologyHandler(svc)

	authMiddleware := *middlewares.NewTopologyAuthMiddleware(
		cfg.Lifecycle.PublicEndpoint,
		tokenValidator,
	)

	server := api.New(cfg.Server, authMiddleware)
	server.RegisterRoutes(app, th)

	runCtx, runCancel := context.WithCancel(context.Background())
	defer runCancel()

	if consumer != nil {

		go func() {
			log := logger.GetLoggerThreadId("KAFKA-CONSUMER")
			log.Info("starting kafka consumer")

			if err := consumer.Run(runCtx); err != nil {
				log.WithError(err).Error("kafka consumer stopped")
			}
		}()
	}

	go func(ctx context.Context) {
		log := logger.GetLoggerThreadId("DISCOVERY")
		log.Info("starting discovery component")

		discoveryComponent.Run(ctx)
	}(runCtx)

	lifecycleService.Start(runCtx)

	err = server.Start(app)
	if err != nil {
		log.WithError(err).Fatal("failed to start http server")
	}
	_ = app.Shutdown()
	if consumer != nil {
		_ = consumer.Close()
	}
}

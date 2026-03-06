package main

import (
	"context"
	"crypto/x509"
	"fmt"
	"os"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/client"

	serviceclient "github.com/router-architects/ra-openlan-nw-topology/adapters/httpclient"
	"github.com/router-architects/ra-openlan-nw-topology/internal/api"
	"github.com/router-architects/ra-openlan-nw-topology/internal/api/handlers"
	"github.com/router-architects/ra-openlan-nw-topology/internal/api/middlewares"
	"github.com/router-architects/ra-openlan-nw-topology/internal/config"
	"github.com/router-architects/ra-openlan-nw-topology/internal/gateway/analytics"
	"github.com/router-architects/ra-openlan-nw-topology/internal/gateway/security"
	"github.com/router-architects/ra-openlan-nw-topology/internal/services"

	servicediscovery "github.com/routerarchitects/ow-common-mods/servicediscovery"
	logger "github.com/routerarchitects/ra-common-mods/logger"
	logger_routes "github.com/routerarchitects/ra-common-mods/logger-routes"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}

	logCfg := config.GetLoggerConfig(*cfg)
	log, shutdown, err := logger.Init(logCfg)
	if err != nil {
		panic(err)
	}
	defer shutdown()

	log = logger.Subsystem("server")
	log.InfoContext(context.Background(), "subsys log")

	if log == nil {
		panic("failed to init logger")
	}

	log.Info("successfully init log level")
	dicoveryConfig := config.GetDiscoveryConfig(*cfg)
	kafkaConfig := config.GetKafkaConfig(*cfg)

	log = logger.Subsystem("Service-discovery")
	discovery, err := servicediscovery.New(dicoveryConfig, kafkaConfig, log)
	if err != nil {
		log.Error("failed to create discovery", "error", err)
	}

	fiberClient := client.New()
	fiberClient.SetTimeout(5 * time.Second)
	if cfg.Server.TLS_ROOTCA != "" {
		pemBytes, err := os.ReadFile(cfg.Server.TLS_ROOTCA)
		if err != nil {
			log.Error("failed to read TLS root CA cert")
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pemBytes) {
			log.Error("failed to parse token validation CA cert")
		}
		fiberClient.TLSConfig().RootCAs = pool
	}

	log = logger.Subsystem("http-client")

	OpenAPIRequestClient := serviceclient.NewOpenApiRequest(
		fiberClient,
		serviceclient.OpenAPIRequestConfig{
			Timeout: 3 * time.Second,
		},
		log,
	)

	log = logger.Subsystem("gateway")

	tokenValidator := security.NewTokenValidator(
		discovery,
		OpenAPIRequestClient,
		log,
	)
	analyticsClient := analytics.NewAnalyticsClient(OpenAPIRequestClient, discovery, log)

	log = logger.Subsystem("topology-services")
	svc := services.NewTopologyService(analyticsClient, log)

	app := fiber.New(fiber.Config{
		ReadTimeout:  time.Second * 10,
		WriteTimeout: time.Second * 15,
	})
	logger_routes.RegisterFiberRoutes(app.Group("/logger"))

	th := handlers.NewTopologyHandler(svc)

	log = logger.Subsystem("middleware")
	authMiddleware := *middlewares.NewTopologyAuthMiddleware(
		cfg.Discovery.PublicEndpoint,
		tokenValidator,
		log,
	)

	server := api.New(cfg.Server, authMiddleware, log)
	server.RegisterRoutes(app, th)

	if err := discovery.Start(context.Background()); err != nil {
		panic("failed to start service discovery")
	}

	err = server.Start(app)
	if err != nil {
		panic(fmt.Sprintf("failed to start server : %v", err))
	}
	_ = app.Shutdown()
}

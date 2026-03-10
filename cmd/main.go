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
	rootLog, shutdown, err := logger.Init(logCfg)
	if err != nil {
		panic(err)
	}
	defer shutdown()

	if rootLog == nil {
		panic(fmt.Sprintf("logger init returned nil logger"))
	}
	rootLog.InfoContext(context.Background(), "logger initialized")

	serverLog := logger.Subsystem("server")
	discoveryLog := logger.Subsystem("service-discovery")
	httpClientLog := logger.Subsystem("http-client")
	gatewayLog := logger.Subsystem("gateway")
	serviceLog := logger.Subsystem("topology-service")
	middlewareLog := logger.Subsystem("middleware")

	discoveryConfig := config.GetDiscoveryConfig(*cfg)
	kafkaConfig := config.GetKafkaConfig(*cfg)

	discovery, err := servicediscovery.New(discoveryConfig, kafkaConfig, discoveryLog)
	if err != nil {
		panic(fmt.Sprintf("create service discovery: %v", err))
	}

	fiberClient := client.New()
	fiberClient.SetTimeout(5 * time.Second)

	if cfg.Server.TLS_ROOTCA != "" {
		pemBytes, err := os.ReadFile(cfg.Server.TLS_ROOTCA)
		if err != nil {
			panic(fmt.Sprintf("read TLS root CA %q: %v", cfg.Server.TLS_ROOTCA, err))
		}

		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pemBytes) {
			panic(fmt.Sprintf("parse TLS root CA %q: invalid PEM", cfg.Server.TLS_ROOTCA))
		}

		fiberClient.TLSConfig().RootCAs = pool
	}

	openAPIRequestClient := serviceclient.NewOpenApiRequest(
		fiberClient,
		serviceclient.OpenAPIRequestConfig{
			Timeout: 3 * time.Second,
		},
		httpClientLog,
	)

	tokenValidator := security.NewTokenValidator(
		discovery,
		openAPIRequestClient,
		gatewayLog,
	)

	analyticsClient := analytics.NewAnalyticsClient(
		openAPIRequestClient,
		discovery,
		gatewayLog,
	)

	svc := services.NewTopologyService(analyticsClient, serviceLog)
	topologyHandler := handlers.NewTopologyHandler(svc)

	authMiddleware := *middlewares.NewTopologyAuthMiddleware(
		cfg.Discovery.PublicEndpoint,
		tokenValidator,
		middlewareLog,
	)

	server := api.New(cfg.Server, authMiddleware, serverLog)

	app := fiber.New(fiber.Config{
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 15 * time.Second,
	})

	server.RegisterCommon(app)
	logger_routes.RegisterFiberRoutes(app.Group("/logger"))
	server.RegisterRoutes(app, topologyHandler)

	if err := discovery.Start(context.Background()); err != nil {
		panic(fmt.Sprintf("failed to start service discovery : %v", err))
	}

	if err := server.Start(app); err != nil {
		panic(fmt.Sprintf("failed to start server : %v", err))
	}

}

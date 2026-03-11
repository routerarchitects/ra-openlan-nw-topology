package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/gofiber/fiber/v3"

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

	openAPIRequestClient := serviceclient.NewOpenApiRequest(
		serviceclient.OpenAPIRequestConfig{
			Timeout: 15 * time.Second,
		},
		httpClientLog,
		cfg.Server.TLS_ROOTCA,
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

	publicApp := fiber.New(fiber.Config{
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 15 * time.Second,
	})
	privateApp := fiber.New(fiber.Config{
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 15 * time.Second,
	})

	server.RegisterMiddlewares(publicApp, privateApp, authMiddleware)
	server.RegisterRoutes(publicApp, privateApp, topologyHandler)

	if err := discovery.Start(context.Background()); err != nil {
		panic(fmt.Sprintf("failed to start service discovery : %v", err))
	}

	if err := server.Start(publicApp, privateApp); err != nil {
		panic(fmt.Sprintf("failed to start server : %v", err))
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)

	<-stop

	if err := publicApp.Shutdown(); err != nil {
		rootLog.Error("Forced shutdown")
	}

	if err := privateApp.Shutdown(); err != nil {
		rootLog.Error("Forced shutdown")
	}

}

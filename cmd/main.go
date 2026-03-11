package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/gofiber/fiber/v3"

	serviceanalytics "github.com/router-architects/ra-openlan-nw-topology/adapters/service_rpc/analytics"
	serviceowsec "github.com/router-architects/ra-openlan-nw-topology/adapters/service_rpc/owsec"
	"github.com/router-architects/ra-openlan-nw-topology/internal/api"
	"github.com/router-architects/ra-openlan-nw-topology/internal/api/handlers"
	"github.com/router-architects/ra-openlan-nw-topology/internal/api/middlewares"
	"github.com/router-architects/ra-openlan-nw-topology/internal/config"
	"github.com/router-architects/ra-openlan-nw-topology/internal/services"

	servicediscovery "github.com/routerarchitects/ow-common-mods/servicediscovery"
	logger "github.com/routerarchitects/ra-common-mods/logger"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}

	rootLog, shutdown, err := logger.Init(cfg.Logger.ModuleConfig())
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
	owsecrpcLog := logger.Subsystem("owsec-service-rpc")
	analyticsrpcLog := logger.Subsystem("analytics-service-rpc")
	serviceLog := logger.Subsystem("topology-service")
	middlewareLog := logger.Subsystem("middleware")

	discoveryConfig := cfg.Discovery.ModuleConfig()
	kafkaConfig := cfg.Kafka.ModuleConfig()

	discovery, err := servicediscovery.New(discoveryConfig, kafkaConfig, discoveryLog)
	if err != nil {
		panic(fmt.Sprintf("create service discovery: %v", err))
	}

	tokenValidator := serviceowsec.NewValidator(
		discovery,
		owsecrpcLog,
		cfg.Server.TLS_ROOTCA,
		15*time.Second,
		cfg.Logger.ServiceName,
	)

	analyticsClient := serviceanalytics.NewClient(
		discovery,
		analyticsrpcLog,
		cfg.Server.TLS_ROOTCA,
		15*time.Second,
		cfg.Logger.ServiceName,
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

	if err := discovery.Stop(context.Background()); err != nil {
		rootLog.Error("failed to stop service discovery", "error", err)
	}

}

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v3"

	service_rpc "github.com/router-architects/ra-openlan-nw-topology/adapters/service_rpc"
	"github.com/router-architects/ra-openlan-nw-topology/internal/api"
	"github.com/router-architects/ra-openlan-nw-topology/internal/api/handlers"
	"github.com/router-architects/ra-openlan-nw-topology/internal/api/middlewares"
	"github.com/router-architects/ra-openlan-nw-topology/internal/config"
	"github.com/router-architects/ra-openlan-nw-topology/internal/services"

	servicediscovery "github.com/routerarchitects/ow-common-mods/servicediscovery"
	Subsystem "github.com/routerarchitects/ow-common-mods/system-routes"
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
	ctx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()
	rootLog.InfoContext(context.Background(), "logger initialized")

	serverLog := logger.Subsystem("server")
	discoveryLog := logger.Subsystem("service-discovery")
	serviceRpcLog := logger.Subsystem("service-rpc")
	serviceLog := logger.Subsystem("topology-service")
	middlewareLog := logger.Subsystem("middleware")

	discoveryConfig := cfg.Discovery.ModuleConfig()
	kafkaConfig := cfg.Kafka.ModuleConfig()
	subsystemConfig := cfg.Subsystem.ModuleConfig(cfg.Server)

	discovery, err := servicediscovery.New(discoveryConfig, kafkaConfig, discoveryLog)
	if err != nil {
		panic(fmt.Sprintf("create service discovery: %v", err))
	}

	rpcFactory := service_rpc.NewServiceRpc(
		discovery,
		service_rpc.ServiceRpcConfig{
			TLSRootCA:    cfg.Server.TLS_ROOTCA,
			Timeout:      15 * time.Second,
			InternalName: cfg.Logger.ServiceName,
		},
		serviceRpcLog,
	)

	tokenValidator := rpcFactory.Validator()

	analyticsClient := rpcFactory.AnalyticsClient()

	svc := services.NewTopologyService(analyticsClient, serviceLog)
	topologyHandler := handlers.NewTopologyHandler(svc)

	authMiddleware := *middlewares.NewTopologyAuthMiddleware(
		discoveryConfig.InstanceKey,
		tokenValidator,
		middlewareLog,
	)

	subsystemRoutes := Subsystem.NewSubsytems(subsystemConfig)

	server := api.New(cfg.Server, authMiddleware, serverLog, subsystemRoutes)

	appConfig := fiber.Config{
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 15 * time.Second,
	}
	publicApp := fiber.New(appConfig)
	privateApp := fiber.New(appConfig)

	server.RegisterMiddlewares(publicApp, privateApp)
	server.RegisterRoutes(publicApp, privateApp, topologyHandler)

	if err := discovery.Start(ctx); err != nil {
		panic(fmt.Sprintf("failed to start service discovery : %v", err))
	}

	serverErrCh, err := server.Start(ctx, publicApp, privateApp)
	if err != nil {
		panic(fmt.Sprintf("failed to start server : %v", err))
	}

	select {
	case <-ctx.Done():
		rootLog.Info("shutdown signal received")
	case err := <-serverErrCh:
		if err != nil {
			rootLog.Error("server exited unexpectedly", "error", err)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := publicApp.Shutdown(); err != nil {
		rootLog.Error("forced public shutdown", "error", err)
	}

	if err := privateApp.Shutdown(); err != nil {
		rootLog.Error("forced private shutdown", "error", err)
	}

	if err := discovery.Stop(shutdownCtx); err != nil {
		rootLog.Error("failed to stop service discovery", "error", err)
	}

}

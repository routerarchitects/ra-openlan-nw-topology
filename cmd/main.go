package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/router-architects/ra-openlan-nw-topology/internal/config"
	apphttp "github.com/router-architects/ra-openlan-nw-topology/internal/http"
	"github.com/router-architects/ra-openlan-nw-topology/internal/services"
	"github.com/routerarchitects/ow-common-mods/fiber/middleware/auth"
	"github.com/routerarchitects/ow-common-mods/servicerpc"

	"github.com/routerarchitects/ow-common-mods/servicediscovery"
	"github.com/routerarchitects/ra-common-mods/logger"
)

func main() {
	ctx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}

	rootLog, loggerShutdown, err := logger.Init(cfg.Logger.Config)
	if err != nil {
		panic(err)
	}
	defer loggerShutdown()

	if rootLog == nil {
		panic("logger init returned nil logger")
	}

	rootLog.InfoContext(ctx, "logger initialized")

	discoveryConfig := cfg.Discovery.Config
	kafkaConfig := cfg.Kafka.Config
	subsystemConfig := cfg.Subsystem.Config

	discovery, err := servicediscovery.New(discoveryConfig, kafkaConfig, logger.Subsystem("service-discovery"))
	if err != nil {
		panic(fmt.Sprintf("create service discovery: %v", err))
	}

	rpcFactory, err := servicerpc.NewServiceRpc(
		discovery,
		servicerpc.ServiceRpcConfig{
			TLSRootCA:    cfg.Server.TLS_ROOTCA,
			InternalName: cfg.Discovery.PublicEndpoint,
		},
		logger.Subsystem("service-rpc"),
	)
	if err != nil {
		panic(fmt.Sprintf("create service RPC factory: %v", err))
	}
	tokenValidator, err := rpcFactory.SecurityClient()
	if err != nil {
		panic(fmt.Sprintf("create security client: %v", err))
	}
	analyticsClient, err := rpcFactory.AnalyticsClient()
	if err != nil {
		panic(fmt.Sprintf("create analytics client: %v", err))
	}
	svc := services.NewTopologyService(analyticsClient)

	publicAuthConfig := auth.PublicAuthConfig{}
	privateAuthConfig := auth.InternalAPIKeyConfig{
		ExpectedAPIKey: cfg.Discovery.InstanceKey,
	}
	module, err := apphttp.NewModule(apphttp.Dependencies{
		ServerLogger:      logger.Subsystem("server"),
		ServerConfig:      cfg.Server,
		SubsystemConfig:   subsystemConfig,
		TopologyService:   svc,
		PublicAuthConfig:  publicAuthConfig,
		PrivateAuthConfig: privateAuthConfig,
		TokenValidator:    tokenValidator,
	})
	if err != nil {
		panic(fmt.Sprintf("create api module: %v", err))
	}

	if err := discovery.Start(ctx); err != nil {
		panic(fmt.Sprintf("failed to start service discovery : %v", err))
	}

	serverErrCh, err := module.Start(ctx)
	if err != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if stopErr := discovery.Stop(shutdownCtx); stopErr != nil {
			rootLog.Error("failed to stop service discovery after server startup failure", "error", stopErr)
		}
		panic(fmt.Sprintf("failed to start server : %v", err))
	}

	select {
	case <-ctx.Done():
		rootLog.Info("shutdown signal received")
	case err := <-serverErrCh:
		if err != nil {
			rootLog.Error("server exited unexpectedly", "error", err)
		} else {
			rootLog.Info("server listener exited")
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := module.Shutdown(); err != nil {
		rootLog.Error("forced app shutdown", "error", err)
	}

	if err := discovery.Stop(shutdownCtx); err != nil {
		rootLog.Error("failed to stop service discovery", "error", err)
	}

}

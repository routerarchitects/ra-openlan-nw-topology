package http

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/router-architects/ra-openlan-nw-topology/internal/config"
	"github.com/router-architects/ra-openlan-nw-topology/internal/http/handlers"
	"github.com/router-architects/ra-openlan-nw-topology/internal/http/middleware"
	"github.com/router-architects/ra-openlan-nw-topology/internal/http/router"
	"github.com/routerarchitects/ow-common-mods/fiber/middleware/auth"
	subsystemroutes "github.com/routerarchitects/ow-common-mods/fiber/system-routes"
	"github.com/routerarchitects/ow-common-mods/servicerpc/owsec"
)

type Dependencies struct {
	ServerLogger      *slog.Logger
	ServerConfig      config.ServerConfig
	SubsystemConfig   subsystemroutes.Config
	TopologyService   handlers.TopologyService
	PublicAuthConfig  auth.PublicAuthConfig
	PrivateAuthConfig auth.InternalAPIKeyConfig
	TokenValidator    *owsec.SecurityClient
}

type Module struct {
	server     *Server
	publicApp  *fiber.App
	privateApp *fiber.App
}

func NewModule(deps Dependencies) (*Module, error) {
	if deps.TopologyService == nil {
		return nil, fmt.Errorf("topology service is required")
	}

	authMiddleware, err := middleware.NewTopologyAuth(
		deps.PublicAuthConfig,
		deps.PrivateAuthConfig,
		deps.TokenValidator,
	)
	if err != nil {
		return nil, err
	}

	appConfig := fiber.Config{
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 15 * time.Second,
	}
	publicApp := fiber.New(appConfig)
	privateApp := fiber.New(appConfig)

	middleware.RegisterPublicCORS(publicApp)
	middleware.RegisterRequestLog(publicApp, deps.ServerLogger)
	middleware.RegisterRequestLog(privateApp, deps.ServerLogger)

	topologyHandler := handlers.NewTopologyHandler(deps.TopologyService)
	router.RegisterPublic(publicApp, router.PublicDeps{
		AuthHandler: authMiddleware.GetPublicAuthHandler(),
		Topology:    topologyHandler,
		Subsystem:   deps.SubsystemConfig,
	})

	router.RegisterPrivate(privateApp, router.PrivateDeps{
		AuthHandler: authMiddleware.GetPrivateAuthHandler(),
		Topology:    topologyHandler,
		Subsystem:   deps.SubsystemConfig,
	})

	return &Module{
		server:     NewServer(deps.ServerConfig, deps.ServerLogger),
		publicApp:  publicApp,
		privateApp: privateApp,
	}, nil
}

func (m *Module) Start(ctx context.Context) (<-chan error, error) {
	return m.server.Start(ctx, m.publicApp, m.privateApp)
}

func (m *Module) Shutdown() error {
	var errs []error
	if err := m.publicApp.Shutdown(); err != nil {
		errs = append(errs, fmt.Errorf("public app shutdown: %w", err))
	}
	if err := m.privateApp.Shutdown(); err != nil {
		errs = append(errs, fmt.Errorf("private app shutdown: %w", err))
	}
	return errors.Join(errs...)
}

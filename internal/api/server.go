package api

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	subsystemmodules "github.com/routerarchitects/ow-common-mods/system-routes"

	"github.com/router-architects/ra-openlan-nw-topology/adapters/apperrors"
	"github.com/router-architects/ra-openlan-nw-topology/internal/api/middlewares"
	"github.com/router-architects/ra-openlan-nw-topology/internal/config"
)

type Server struct {
	Crt            string
	Key            string
	Port           int
	PrivatePort    int
	AuthMiddleware middlewares.TopologyAuthMiddleware
	logger         *slog.Logger
	subsystem      *subsystemmodules.Routes
}

func New(cfg config.ServerConfig, authMiddleware middlewares.TopologyAuthMiddleware, logger *slog.Logger, subsystem *subsystemmodules.Routes) *Server {

	server := Server{
		Crt:            cfg.TLS_CERT,
		Key:            cfg.TLS_KEY,
		Port:           cfg.HTTPPort,
		PrivatePort:    cfg.PrivatePort,
		AuthMiddleware: authMiddleware,
		logger:         logger,
		subsystem:      subsystem,
	}
	return &server
}

func (s *Server) RegisterMiddlewares(publicApp *fiber.App, privateApp *fiber.App) {
	publicApp.Use(cors.New(cors.Config{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{"GET,POST,PUT,DELETE,OPTIONS"},
		AllowHeaders: []string{"Origin", " Content-Type", " Accept", " Authorization"},
	}))
	publicApp.Use(s.AuthMiddleware.TopologyPublicAuth)
	publicApp.Use(middlewares.RequestLogger(s.logger))

	privateApp.Use(s.AuthMiddleware.TopologyPrivateAuth)
	privateApp.Use(middlewares.RequestLogger(s.logger))
}

func (s *Server) Start(ctx context.Context, publicApp *fiber.App, privateApp *fiber.App) (<-chan error, error) {

	if s.Port <= 0 || s.PrivatePort <= 0 {
		return nil, apperrors.WrapError(apperrors.CodeInternal, "invalid ports", nil)
	}
	if s.Port == s.PrivatePort {
		return nil, apperrors.WrapError(apperrors.CodeInternal, "public and private ports must be different", nil)
	}

	crt := s.Crt
	key := s.Key
	if crt == "" || key == "" {
		return nil, apperrors.WrapError(apperrors.CodeInternal, "tls certificate and key must not be empty", nil)
	}

	if _, err := os.Stat(crt); err != nil {
		return nil, err
	}
	if _, err := os.Stat(key); err != nil {
		return nil, err
	}

	cert, err := tls.LoadX509KeyPair(crt, key)
	if err != nil {
		return nil, err
	}
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	ln, err := tls.Listen("tcp", fmt.Sprintf(":%d", s.Port), tlsConfig)
	if err != nil {
		return nil, err
	}

	lnPrivate, err := tls.Listen("tcp", fmt.Sprintf(":%d", s.PrivatePort), tlsConfig)
	if err != nil {
		_ = ln.Close()
		return nil, err
	}

	errCh := make(chan error, 2)

	go func() {
		if err := publicApp.Listener(ln); err != nil && !isExpectedListenerClose(ctx, err) {
			errCh <- fmt.Errorf("public server stopped on port %d: %w", s.Port, err)
		}
	}()

	go func() {
		if err := privateApp.Listener(lnPrivate); err != nil && !isExpectedListenerClose(ctx, err) {
			errCh <- fmt.Errorf("private server stopped on port %d: %w", s.PrivatePort, err)
		}
	}()

	s.logger.Info("servers started", "public_port", s.Port, "private_port", s.PrivatePort)
	return errCh, nil
}

func isExpectedListenerClose(ctx context.Context, err error) bool {
	if err == nil {
		return true
	}

	if errors.Is(err, net.ErrClosed) {
		return true
	}

	return ctx != nil && ctx.Err() != nil
}

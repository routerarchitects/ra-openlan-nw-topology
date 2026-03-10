package api

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/gofiber/fiber/v3"

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
}

func New(cfg config.ServerConfig, authMiddleware middlewares.TopologyAuthMiddleware, logger *slog.Logger) *Server {

	server := Server{
		Crt:            cfg.TLS_CERT,
		Key:            cfg.TLS_KEY,
		Port:           cfg.HTTPPort,
		PrivatePort:    cfg.PrivatePort,
		AuthMiddleware: authMiddleware,
		logger:         logger,
	}
	return &server
}

func (s *Server) RegisterCommon(app *fiber.App) {
	app.Get("/livez", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	app.Use(middlewares.RequestLogger(s.logger))
}

func (s *Server) Start(app *fiber.App) error {
	crt := s.Crt
	key := s.Key
	if crt == "" || key == "" {
		return apperrors.WrapError(apperrors.CodeInternal, "tls certificate and key must not be empty", nil)
	}

	if _, err := os.Stat(crt); err != nil {
		return err
	}
	if _, err := os.Stat(key); err != nil {
		return err
	}

	cert, err := tls.LoadX509KeyPair(crt, key)
	if err != nil {
		return err
	}
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	ln, err := tls.Listen("tcp", fmt.Sprintf(":%d", s.Port), tlsConfig)
	if err != nil {
		return err
	}

	lnPrivate, err := tls.Listen("tcp", fmt.Sprintf(":%d", s.PrivatePort), tlsConfig)
	if err != nil {
		_ = ln.Close()
		return err
	}

	errCh := make(chan error, 2)

	go func() {
		if err := app.Listener(ln); err != nil {
			errCh <- err
		}
	}()

	go func() {
		if err := app.Listener(lnPrivate); err != nil {
			errCh <- err
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(stop)

	select {
	case sig := <-stop:
		s.logger.Info("shutdown requested", "signal", sig.String())
	case err := <-errCh:
		if err != nil {
			_ = app.Shutdown()
			return err
		}
	}

	_ = ln.Close()
	_ = lnPrivate.Close()

	if err := app.Shutdown(); err != nil {
		return err
	}

	return nil
}

package http

import (
	"context"
	"crypto/tls"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/router-architects/network-topology-service/internal/apperrors"
	"github.com/router-architects/network-topology-service/internal/config"
	"github.com/router-architects/network-topology-service/internal/http/handlers"
	"github.com/router-architects/network-topology-service/internal/http/middlewares"
	"github.com/router-architects/network-topology-service/internal/logger"
)

type ServerDeps struct {
	APIKey         string
	TopologyWindow time.Duration
	TopologyDrift  time.Duration
}

func New(app *fiber.App, deps ServerDeps, th *handlers.TopologyHandler) *fiber.App {

	app.Get("/livez", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	app.Get("/readyz", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	// middlewares
	app.Use(middlewares.RequestLogger())
	// app.Use(middlewares.APIKeyAuth(deps.APIKey))

	// inject window/drift into context locals for handlers
	app.Use(func(c fiber.Ctx) error {
		c.Locals("topology_window", deps.TopologyWindow)
		c.Locals("topology_drift", deps.TopologyDrift)
		return c.Next()
	})
	return app
}

func (s *ServerDeps) Start(app *fiber.App, cfg config.Config, pool pgxpool.Pool) error {

	crt := cfg.TLS_CERT
	key := cfg.TLS_KEY
	if crt == "" || key == "" {
		logger.GetLogger().WithError(apperrors.WrapError(apperrors.CodeConflict, "TLS_CERT and TLS_KEY must be set", nil)).Fatal("TLS_CERT and TLS_KEY must be set")
	}

	if _, err := os.Stat(crt); err != nil {
		logger.GetLogger().WithError(err).Fatal("TLS cert not found/readable: %s (%v)", crt, err)
	}
	if _, err := os.Stat(key); err != nil {
		logger.GetLogger().WithError(err).Fatal("TLS key not found/readable: %s (%v)", key, err)
	}

	cert, err := tls.LoadX509KeyPair(crt, key)
	if err != nil {
		logger.GetLogger().WithError(err).Fatal("Failed to load X509 key pair (cert=%s key=%s): %v", crt, key, err)
	}
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	port := cfg.HTTPPort
	addr := fmt.Sprintf(":%d", port)

	ln, err := tls.Listen("tcp", addr, tlsConfig)
	if err != nil {
		logger.GetLogger().WithError(err).Fatal("failed to start TLS listener on %s: %v", addr, err)
	}

	// ---------- serve + graceful shutdown ----------
	// log.WithField("addr", addr).Info("Listening (TLS)")

	// Run the Fiber server in its own goroutine
	go func() {
		if err := app.Listener(ln); err != nil {
			logger.GetLogger().WithError(err).Fatal("fiber listener stopped")
		}
	}()

	// return ln, nil
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	// Stop accepting new connections and shut down Fiber
	shutdownCtx, cancel2 := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel2()

	if err := app.Shutdown(); err != nil {
		logger.GetLogger().WithError(err).Error("fiber shutdown error")
	}

	_ = ln.Close()

	// Close DB pool
	pool.Close()

	<-shutdownCtx.Done()
	return nil
}

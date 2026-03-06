package api

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/client"

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

	app := fiber.New()

	app.Get("/livez", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	app.Get("/readyz", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	app.Use(authMiddleware.TopologyAuth)

	app.Use(middlewares.RequestLogger(logger))

	app.Use(func(c fiber.Ctx) error {
		return c.Next()
	})
	server := Server{
		Crt:            cfg.TLS_CERT,
		Key:            cfg.TLS_KEY,
		Port:           cfg.HTTPPort,
		PrivatePort:    cfg.PrivatePort,
		AuthMiddleware: authMiddleware,
	}
	return &server
}

func (s *Server) Start(app *fiber.App) error {
	fiberClient := client.New()
	fiberClient.SetTimeout(5 * time.Second)

	crt := s.Crt
	key := s.Key
	if crt == "" || key == "" {
		panic(fmt.Sprintf("tls certificate and key must not be empty"))
	}

	if _, err := os.Stat(crt); err != nil {
		panic(fmt.Sprintf("tls certificate not found or not readable: %s (%v)", crt, err))
	}
	if _, err := os.Stat(key); err != nil {
		panic(fmt.Sprintf("tls key not found or not readable: %s (%v)", key, err))
	}

	cert, err := tls.LoadX509KeyPair(crt, key)
	if err != nil {
		panic(fmt.Sprintf("failed to load X509 key pair (cert=%s key=%s): %v", crt, key, err))
	}
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	port := s.Port
	addr := fmt.Sprintf(":%d", port)

	ln, err := tls.Listen("tcp", addr, tlsConfig)
	if err != nil {
		panic(fmt.Sprintf("failed to start TLS listener on %s: %v", addr, err))
	}

	// ---------- serve + graceful shutdown ----------
	go func() {
		if err := app.Listener(ln); err != nil {
			s.logger.Error("fiber listener stopped")
		}
	}()

	privatePort := s.PrivatePort
	privateAddr := fmt.Sprintf(":%d", privatePort)

	lnPrivate, err := tls.Listen("tcp", privateAddr, tlsConfig)
	if err != nil {
		panic(fmt.Sprintf("failed to start private TLS listener on %s: %v", privateAddr, err))
	}

	// ---------- serve + graceful shutdown ----------
	go func() {
		if err := app.Listener(lnPrivate); err != nil {
			s.logger.Error("fiber private listener stopped")
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	// Stop accepting new connections and shut down Fiber
	shutdownCtx, cancel2 := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel2()

	if err := app.Shutdown(); err != nil {
		s.logger.Error("fiber shutdown error")
	}

	_ = ln.Close()

	<-shutdownCtx.Done()
	return nil
}

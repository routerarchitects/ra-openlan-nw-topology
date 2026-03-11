package api

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"os"
	"sync"

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

func (s *Server) RegisterMiddlewares(publicApp *fiber.App, privateApp *fiber.App, authMiddleware middlewares.TopologyAuthMiddleware) {
	publicApp.Use(authMiddleware.TopologyPublicAuth)
	publicApp.Use(middlewares.RequestLogger(s.logger))

	privateApp.Use(authMiddleware.TopologyPrivateAuth)
	privateApp.Use(middlewares.RequestLogger(s.logger))
}

func (s *Server) Start(publicApp *fiber.App, privateApp *fiber.App) error {

	if s.Port <= 0 || s.PrivatePort <= 0 {
		return apperrors.WrapError(apperrors.CodeInternal, "invalid ports", nil)
	}
	if s.Port == s.PrivatePort {
		return apperrors.WrapError(apperrors.CodeInternal, "public and private ports must be different", nil)
	}

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
		return err
	}
	defer ln.Close()
	defer lnPrivate.Close()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		if err := publicApp.Listener(ln); err != nil {
			s.logger.Error("public server stopped", "port", s.Port, "error", err)
		}
	}()

	go func() {
		defer wg.Done()
		if err := privateApp.Listener(lnPrivate); err != nil {
			s.logger.Error("private server stopped", "port", s.PrivatePort, "error", err)
		}
	}()

	s.logger.Info("servers started", "public_port", s.Port, "private_port", s.PrivatePort)
	wg.Wait()
	return nil
}

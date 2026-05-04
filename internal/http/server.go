package http

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"

	"github.com/gofiber/fiber/v3"

	"github.com/router-architects/ra-openlan-nw-topology/internal/config"
	"github.com/routerarchitects/ra-common-mods/apperror"
)

type Server struct {
	crt         string
	key         string
	port        int
	privatePort int
	logger      *slog.Logger
}

func NewServer(cfg config.ServerConfig, logger *slog.Logger) *Server {
	return &Server{
		crt:         cfg.TLS_CERT,
		key:         cfg.TLS_KEY,
		port:        cfg.HTTPPort,
		privatePort: cfg.PrivatePort,
		logger:      logger,
	}
}

func (s *Server) Start(ctx context.Context, publicApp *fiber.App, privateApp *fiber.App) (<-chan error, error) {
	if s.port <= 0 || s.privatePort <= 0 {
		return nil, apperror.Wrap(apperror.CodeInternal, "invalid ports", nil)
	}
	if s.port == s.privatePort {
		return nil, apperror.Wrap(apperror.CodeInternal, "public and private ports must be different", nil)
	}

	if s.crt == "" || s.key == "" {
		return nil, apperror.Wrap(apperror.CodeInternal, "tls certificate and key must not be empty", nil)
	}
	if _, err := os.Stat(s.crt); err != nil {
		return nil, err
	}
	if _, err := os.Stat(s.key); err != nil {
		return nil, err
	}

	cert, err := tls.LoadX509KeyPair(s.crt, s.key)
	if err != nil {
		return nil, err
	}
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	publicListener, err := tls.Listen("tcp", fmt.Sprintf(":%d", s.port), tlsConfig)
	if err != nil {
		return nil, err
	}

	privateListener, err := tls.Listen("tcp", fmt.Sprintf(":%d", s.privatePort), tlsConfig)
	if err != nil {
		_ = publicListener.Close()
		return nil, err
	}

	errCh := make(chan error, 2)
	go func() {
		err := publicApp.Listener(publicListener)
		if err != nil && !isExpectedListenerClose(ctx, err) {
			errCh <- fmt.Errorf("public server stopped on port %d: %w", s.port, err)
			return
		}
		errCh <- nil
	}()
	go func() {
		err := privateApp.Listener(privateListener)
		if err != nil && !isExpectedListenerClose(ctx, err) {
			errCh <- fmt.Errorf("private server stopped on port %d: %w", s.privatePort, err)
			return
		}
		errCh <- nil
	}()

	s.logger.Info("servers started", "public_port", s.port, "private_port", s.privatePort)
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

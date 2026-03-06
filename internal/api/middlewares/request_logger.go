package middlewares

import (
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v3"
)

func RequestLogger(logger *slog.Logger) fiber.Handler {
	return func(c fiber.Ctx) error {
		start := time.Now()
		err := c.Next()
		lat := time.Since(start)

		logger.With("method", c.Method(), "path", c.Path(), "status", c.Response().StatusCode(), "latency_ms", lat.Milliseconds()).Info("http_request")

		return err
	}
}

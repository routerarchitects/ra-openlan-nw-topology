package middlewares

import (
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/router-architects/ra-openlan-nw-topology/adapters/logger"
)

func RequestLogger() fiber.Handler {
	return func(c fiber.Ctx) error {
		log := logger.GetLoggerThreadId("SERVER")
		start := time.Now()
		err := c.Next()
		lat := time.Since(start)

		if log != nil {
			log.WithFields(logger.Fields{
				"method":     c.Method(),
				"path":       c.Path(),
				"status":     c.Response().StatusCode(),
				"latency_ms": lat.Milliseconds(),
			}).Info("http_request")
		}

		return err
	}
}

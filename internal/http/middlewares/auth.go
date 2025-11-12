package middlewares

import (
	"context"

	"github.com/router-architects/network-topology-service/internal/apperrors"
	"github.com/router-architects/network-topology-service/internal/logger"
	"github.com/router-architects/network-topology-service/internal/security"

	"github.com/gofiber/fiber/v3"
)

func APIKeyAuth(expected string, validator security.TokenValidator) fiber.Handler {
	return func(c fiber.Ctx) error {
		if expected == "" {
			// no auth configured -> deny
			return c.Status(fiber.StatusUnauthorized).JSON(errBody(apperrors.CodeUnauthorized, "missing API key"))
		}
		got := string(c.Request().Header.Peek("X-API-KEY"))
		internalHeader := c.Get("X-INTERNAL-NAME")
		if internalHeader != "" {
			if got == "" || got != expected {
				logger.GetLogger().WithFields(logger.Fields{
					"path": c.Path(), "method": c.Method(),
				}).Warn("unauthorized request")
				return c.Status(fiber.StatusUnauthorized).JSON(errBody(apperrors.CodeUnauthorized, "unauthorized"))
			}
		} else {
			if validator == nil {
				logger.GetLogger().WithFields(logger.Fields{
					"path": c.Path(), "method": c.Method(),
				}).Error("token validator not configured")
				return c.Status(fiber.StatusUnauthorized).JSON(errBody(apperrors.CodeUnauthorized, "unauthorized"))
			}

			subToken := c.Query("token")
			if subToken == "" {
				subToken = string(c.Request().Header.Peek("X-SUB-TOKEN"))
			}
			if subToken == "" {
				logger.GetLogger().WithFields(logger.Fields{
					"path": c.Path(), "method": c.Method(),
				}).Warn("missing subscription token")
				return c.Status(fiber.StatusUnauthorized).JSON(errBody(apperrors.CodeUnauthorized, "unauthorized"))
			}

			if err := validator.Validate(context.Background(), subToken); err != nil {
				logger.GetLogger().WithFields(logger.Fields{
					"path":   c.Path(),
					"method": c.Method(),
				}).WithError(err).Warn("subscription token validation failed")

				if appErr, ok := err.(*apperrors.Error); ok {
					status := fiber.StatusUnauthorized
					if appErr.Code == apperrors.CodeInternal {
						status = fiber.StatusInternalServerError
					} else if appErr.Code == apperrors.CodeNotFound {
						status = fiber.StatusNotFound
					}
					return c.Status(status).JSON(errBody(appErr.Code, appErr.Message))
				}

				return c.Status(fiber.StatusUnauthorized).JSON(errBody(apperrors.CodeUnauthorized, "unauthorized"))
			}
		}

		return c.Next()
	}
}

func errBody(code apperrors.ErrorCode, msg string) map[string]any {
	return map[string]any{"error": map[string]any{
		"code":    code,
		"message": msg,
	}}
}

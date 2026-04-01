package middlewares

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/router-architects/ra-openlan-nw-topology/adapters/apperrors"
	"github.com/router-architects/ra-openlan-nw-topology/adapters/logger"
)

type TokenValidator interface {
	Validate(ctx context.Context, token string) error
}

type TopologyAuthMiddleware struct {
	PublicEndpoint string
	TokenValidator TokenValidator
}

func NewTopologyAuthMiddleware(publicEndPoint string, validator TokenValidator) *TopologyAuthMiddleware {
	return &TopologyAuthMiddleware{
		PublicEndpoint: publicEndPoint,
		TokenValidator: validator,
	}
}

func (t *TopologyAuthMiddleware) TopologyAuth(c fiber.Ctx) error {
	log := logger.GetLoggerThreadId("SERVER")
	if t.PublicEndpoint == "" {
		fmt.Printf("apiKey name : %s\n", t.PublicEndpoint)
		return writeAuthError(c, apperrors.CodeUnauthorized)
	}
	got := string(c.Request().Header.Peek("X-API-KEY"))
	internalHeader := c.Get("X-INTERNAL-NAME")
	if internalHeader != "" {
		if got == "" || got != sha256Hex(t.PublicEndpoint) {
			return writeAuthError(c, apperrors.CodeUnauthorized)
		}
	} else {
		if t.TokenValidator == nil {
			return writeAuthError(c, apperrors.CodeUnauthorized)
		}

		authHeader := c.Get("Authorization", "")
		if authHeader == "" {
			return writeAuthError(c, apperrors.CodeUnauthorized)
		}

		subToken := strings.TrimSpace(authHeader)
		const bearerPrefix = "Bearer "
		if strings.HasPrefix(subToken, bearerPrefix) {
			subToken = strings.TrimSpace(subToken[len(bearerPrefix):])
		}

		if err := t.TokenValidator.Validate(c.Context(), subToken); err != nil {
			if log != nil {
				log.WithFields(logger.Fields{
					"path":   c.Path(),
					"method": c.Method(),
				}).WithError(err).Warn("subscription token validation failed")
			}

			if appErr, ok := err.(*apperrors.Error); ok {
				return writeAuthError(c, appErr.Code)
			}

			return writeAuthError(c, apperrors.CodeUnauthorized)
		}
	}

	return c.Next()
	// }
}

func writeAuthError(c fiber.Ctx, code apperrors.ErrorCode) error {
	info := apperrors.GetHTTPErrorInfo(code)
	body := map[string]any{
		"ErrorCode":        info.Status,
		"ErrorDescription": fmt.Sprintf("%d: %s", info.Status, info.Description),
		"ErrorDetails":     c.Method(),
	}
	return c.Status(info.Status).JSON(body)
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

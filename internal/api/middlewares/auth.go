package middlewares

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/router-architects/ra-openlan-nw-topology/adapters/apperrors"
)

type TokenValidator interface {
	Validate(ctx context.Context, token string) error
}

type TopologyAuthMiddleware struct {
	PublicEndpoint string
	TokenValidator TokenValidator
	logger         *slog.Logger
}

func NewTopologyAuthMiddleware(publicEndPoint string, validator TokenValidator, logger *slog.Logger) *TopologyAuthMiddleware {
	return &TopologyAuthMiddleware{
		PublicEndpoint: publicEndPoint,
		TokenValidator: validator,
		logger:         logger,
	}
}

func (t *TopologyAuthMiddleware) TopologyAuth(c fiber.Ctx) error {

	got := string(c.Request().Header.Peek("X-API-KEY"))
	internalHeader := c.Get("X-INTERNAL-NAME")
	if internalHeader != "" {
		if got == "" || got != sha256Hex(t.PublicEndpoint) {
			t.logger.Error("auth internal header mismatch")
			return writeAuthError(c, apperrors.CodeUnauthorized)
		}
	} else {
		if t.TokenValidator == nil {
			return writeAuthError(c, apperrors.CodeUnauthorized)
		}

		authHeader := c.Get("Authorization", "")
		if authHeader == "" {
			t.logger.Error("auth header missing")
			return writeAuthError(c, apperrors.CodeUnauthorized)
		}

		subToken := strings.TrimSpace(authHeader)
		const bearerPrefix = "Bearer "
		if strings.HasPrefix(subToken, bearerPrefix) {
			subToken = strings.TrimSpace(subToken[len(bearerPrefix):])
		}

		if err := t.TokenValidator.Validate(c.Context(), subToken); err != nil {

			if appErr, ok := err.(*apperrors.Error); ok {
				t.logger.With("code", appErr.Code).Error("auth token validation Failed")
				return writeAuthError(c, appErr.Code)
			}

			t.logger.With("code", apperrors.CodeUnauthorized).Error("auth token validation Failed")
			return writeAuthError(c, apperrors.CodeUnauthorized)
		}
	}

	t.logger.Info("Authentication successfull")
	return c.Next()
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

package middlewares

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/router-architects/ra-openlan-nw-topology/adapters/apperrors"
)

type TokenValidator interface {
	Validate(token string) error
}

type TopologyAuthMiddleware struct {
	InstanceKey    string
	TokenValidator TokenValidator
	logger         *slog.Logger
}

func NewTopologyAuthMiddleware(instanceKey string, validator TokenValidator, logger *slog.Logger) *TopologyAuthMiddleware {
	return &TopologyAuthMiddleware{
		InstanceKey:    strings.TrimSpace(instanceKey),
		TokenValidator: validator,
		logger:         logger,
	}
}

func (t *TopologyAuthMiddleware) TopologyPublicAuth(c fiber.Ctx) error {

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

	if err := t.TokenValidator.Validate(subToken); err != nil {

		if appErr, ok := err.(*apperrors.Error); ok {
			t.logger.With("code", appErr.Code).Error("auth token validation Failed")
			return writeAuthError(c, appErr.Code)
		}

		t.logger.With("code", apperrors.CodeUnauthorized).Error("auth token validation Failed")
		return writeAuthError(c, apperrors.CodeUnauthorized)
	}
	// }

	t.logger.Info("Authentication successfull")
	return c.Next()
}

func (t *TopologyAuthMiddleware) TopologyPrivateAuth(c fiber.Ctx) error {
	internalHeader := strings.TrimSpace(c.Get("X-INTERNAL-NAME"))
	if internalHeader == "" {
		return writeAuthError(c, apperrors.CodeUnauthorized)
	}

	got := strings.TrimSpace(c.Get("X-API-KEY"))
	if got == "" {
		return writeAuthError(c, apperrors.CodeUnauthorized)
	}

	if t.InstanceKey == "" || got != t.InstanceKey {
		return writeAuthError(c, apperrors.CodeUnauthorized)
	}

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

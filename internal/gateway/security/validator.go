package security

import (
	"context"
	"log/slog"
	"net/url"
	"strings"

	"github.com/gofiber/fiber/v3"
	servicediscovery "github.com/routerarchitects/ow-common-mods/servicediscovery"

	"github.com/router-architects/ra-openlan-nw-topology/adapters/apperrors"

	"github.com/router-architects/ra-openlan-nw-topology/internal/gateway"
)

// TokenValidator validates subscription tokens against an upstream security service.

type owsecValidator struct {
	store  *servicediscovery.Discovery
	client gateway.OpenAPIRequestClient
	logger *slog.Logger
}

const (
	owsecService = "owsec"
)

func NewTokenValidator(discovery *servicediscovery.Discovery, client gateway.OpenAPIRequestClient, logger *slog.Logger) *owsecValidator {

	return &owsecValidator{
		store:  discovery,
		client: client,
		logger: logger,
	}
}

func (v *owsecValidator) Validate(ctx context.Context, rawToken string) error {
	token := strings.TrimSpace(rawToken)
	if token == "" {
		info := apperrors.GetHTTPErrorInfo(apperrors.CodeUnauthorized)
		return apperrors.WrapError(apperrors.CodeUnauthorized, info.Description, nil)
	}

	services := v.store.Store().GetServiceInstances(owsecService)

	validateSubTokenURL := "/api/v1/validateSubToken?token=" + url.QueryEscape(token)

	resp, err := v.client.Do(ctx, fiber.MethodGet, owsecService, validateSubTokenURL, nil, services)

	if resp != nil {
		defer resp.Close()
	}

	if err != nil || resp == nil || resp.StatusCode() != fiber.StatusOK {
		validateTokenURL := "/api/v1/validateToken?token=" + url.QueryEscape(token)

		fallbackResp, err := v.client.Do(ctx, fiber.MethodGet, owsecService, validateTokenURL, nil, services)
		if fallbackResp != nil {
			defer fallbackResp.Close()
		}
		if err != nil {
			v.logger.With("Service", owsecService, "url", validateTokenURL, "operation", "validateToken").Error("validation request Failed")
			info := apperrors.GetHTTPErrorInfo(apperrors.CodeUnauthorized)
			return apperrors.WrapError(apperrors.CodeUnauthorized, info.Description, err)
		}
		if fallbackResp == nil || fallbackResp.StatusCode() != fiber.StatusOK {
			info := apperrors.GetHTTPErrorInfo(apperrors.CodeUnauthorized)
			return apperrors.WrapError(apperrors.CodeUnauthorized, info.Description, nil)
		}
	}

	return nil
}

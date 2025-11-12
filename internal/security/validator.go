package security

import (
	"context"
	"net/url"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/client"

	"github.com/router-architects/network-topology-service/internal/apperrors"
	"github.com/router-architects/network-topology-service/internal/logger"
	"github.com/router-architects/network-topology-service/internal/store"
)

// TokenValidator validates subscription tokens against an upstream security service.
type TokenValidator interface {
	Validate(ctx context.Context, token string) error
}

// ValidatorConfig tunes runtime behavior of the OWSEC token validator.
type ValidatorConfig struct {
	Timeout             time.Duration
	InternalServiceName string
}

type owsecValidator struct {
	store        *store.DiscoveryStore
	client       *client.Client
	timeout      time.Duration
	internalName string
}

const (
	defaultTimeout = 3 * time.Second
	owsecService   = "owsec"
)

// NewTokenValidator constructs a TokenValidator that calls the owsec /validateSubToken API.
func NewTokenValidator(store *store.DiscoveryStore, client *client.Client, cfg ValidatorConfig) TokenValidator {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}

	internalName := strings.TrimSpace(cfg.InternalServiceName)
	if internalName == "" {
		internalName = "nw-topology-service"
	}

	return &owsecValidator{
		store:        store,
		client:       client,
		timeout:      timeout,
		internalName: internalName,
	}
}

func (v *owsecValidator) Validate(ctx context.Context, rawToken string) error {
	token := strings.TrimSpace(rawToken)
	if token == "" {
		return apperrors.WrapError(apperrors.CodeUnauthorized, "missing subscription token", nil)
	}

	if v.store == nil || v.client == nil {
		return apperrors.WrapError(apperrors.CodeInternal, "token validator not configured", nil)
	}

	if ctx == nil {
		ctx = context.Background()
	}

	evt, ok := v.store.Get(owsecService)
	if !ok {
		return apperrors.WrapError(apperrors.CodeNotFound, "NOT_FOUND", nil)
	}

	endpoint := strings.TrimSpace(evt.PrivateEndPoint)
	if endpoint == "" {
		return apperrors.WrapError(apperrors.CodeUnauthorized, "owsec discovery event missing private endpoint", nil)
	}

	apiKey := strings.TrimSpace(evt.Key)
	if apiKey == "" {
		return apperrors.WrapError(apperrors.CodeUnauthorized, "owsec discovery event missing key", nil)
	}

	validateURL := strings.TrimSuffix(endpoint, "/") + "/api/v1/validateSubToken?token=" + url.QueryEscape(token)

	authHeader := token
	if !strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		authHeader = "Bearer " + authHeader
	}

	reqCtx, cancel := context.WithTimeout(ctx, v.timeout)
	defer cancel()

	resp, err := v.client.R().
		SetContext(reqCtx).
		SetTimeout(v.timeout).
		SetHeader(fiber.HeaderAccept, "application/json").
		SetHeader("X-API-KEY", apiKey).
		SetHeader("X-INTERNAL-NAME", v.internalName).
		SetHeader(fiber.HeaderAuthorization, authHeader).
		Get(validateURL)

	if resp != nil {
		defer resp.Close()
	}

	if err != nil || resp == nil || resp.StatusCode() != fiber.StatusOK {
		validateURL := strings.TrimSuffix(endpoint, "/") + "/api/v1/validateToken?token=" + url.QueryEscape(token)
		fallbackResp, err := v.client.R().
			SetContext(reqCtx).
			SetTimeout(v.timeout).
			SetHeader(fiber.HeaderAccept, "application/json").
			SetHeader("X-API-KEY", apiKey).
			SetHeader("X-INTERNAL-NAME", v.internalName).
			SetHeader(fiber.HeaderAuthorization, authHeader).
			Get(validateURL)
		if fallbackResp != nil {
			defer fallbackResp.Close()
		}
		if err != nil {
			logger.GetLogger().WithFields(logger.Fields{
				"service":   owsecService,
				"url":       validateURL,
				"operation": "validateToken",
			}).WithError(err).Error("validateToken request failed")
			return apperrors.WrapError(apperrors.CodeUnauthorized, "unauthorized", err)
		}
		if fallbackResp == nil || fallbackResp.StatusCode() != fiber.StatusOK {
			httpCode := 0
			if fallbackResp != nil {
				httpCode = fallbackResp.StatusCode()
			}
			logger.GetLogger().WithFields(logger.Fields{
				"service":   owsecService,
				"url":       validateURL,
				"httpCode":  httpCode,
				"operation": "validateToken",
			}).Error("validateToken rejected request")
			return apperrors.WrapError(apperrors.CodeUnauthorized, "unauthorized", nil)
		}
	}

	return nil
}

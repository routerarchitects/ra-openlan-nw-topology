package owsec

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/client"
	"github.com/router-architects/ra-openlan-nw-topology/adapters/apperrors"
	"github.com/router-architects/ra-openlan-nw-topology/adapters/service_rpc/common"
	servicediscovery "github.com/routerarchitects/ow-common-mods/servicediscovery"
)

const serviceName = "owsec"

type Validator struct {
	discovery    *servicediscovery.Discovery
	client       *client.Client
	timeout      time.Duration
	internalName string
	logger       *slog.Logger
}

func NewValidator(
	discovery *servicediscovery.Discovery,
	logger *slog.Logger,
	tlsRootCA string,
	timeout time.Duration,
	internalName string,
) *Validator {
	timeout = common.NormalizeTimeout(timeout)

	return &Validator{
		discovery:    discovery,
		client:       common.NewFiberClient(timeout, tlsRootCA),
		timeout:      timeout,
		internalName: common.NormalizeInternalName(internalName),
		logger:       logger,
	}
}

func (v *Validator) Validate(rawToken string) error {
	token := strings.TrimSpace(rawToken)
	if token == "" {
		info := apperrors.GetHTTPErrorInfo(apperrors.CodeUnauthorized)
		return apperrors.WrapError(apperrors.CodeUnauthorized, info.Description, nil)
	}

	resp, err := v.send(context.Background(), "/api/v1/validateSubToken?token="+url.QueryEscape(token))
	if resp != nil {
		defer resp.Close()
	}

	if err == nil && resp != nil && resp.StatusCode() == fiber.StatusOK {
		return nil
	}

	fallbackResp, fallbackErr := v.send(context.Background(), "/api/v1/validateToken?token="+url.QueryEscape(token))
	if fallbackResp != nil {
		defer fallbackResp.Close()
	}

	if fallbackErr != nil {
		v.logger.With("service", serviceName, "operation", "validateToken").Error("validation request failed")
		info := apperrors.GetHTTPErrorInfo(apperrors.CodeUnauthorized)
		return apperrors.WrapError(apperrors.CodeUnauthorized, info.Description, fallbackErr)
	}

	if fallbackResp == nil || fallbackResp.StatusCode() != fiber.StatusOK {
		info := apperrors.GetHTTPErrorInfo(apperrors.CodeUnauthorized)
		return apperrors.WrapError(apperrors.CodeUnauthorized, info.Description, nil)
	}

	return nil
}

func (v *Validator) send(ctx context.Context, endpoint string) (*client.Response, error) {
	service, err := v.resolveService()
	if err != nil {
		return nil, err
	}

	reqCtx, cancel := context.WithTimeout(ctx, v.timeout)
	defer cancel()

	resp, err := v.client.R().
		SetContext(reqCtx).
		SetTimeout(v.timeout).
		SetMethod(fiber.MethodGet).
		SetHeader("X-API-KEY", service.Key).
		SetHeader("X-INTERNAL-NAME", v.internalName).
		SetURL(strings.TrimSuffix(service.PrivateEndPoint, "/") + endpoint).
		Send()
	if err != nil {
		return nil, apperrors.WrapError(apperrors.CodeInternal, "validation request failed", err)
	}

	return resp, nil
}

func (v *Validator) resolveService() (servicediscovery.Instance, error) {
	if v.discovery == nil {
		return servicediscovery.Instance{}, apperrors.WrapError(apperrors.CodeInternal, "service discovery is not configured", nil)
	}

	services := v.discovery.Store().GetServiceInstances(serviceName)
	if len(services) == 0 {
		return servicediscovery.Instance{}, apperrors.WrapError(apperrors.CodeNotFound, http.StatusText(http.StatusNotFound), nil)
	}

	return services[0], nil
}

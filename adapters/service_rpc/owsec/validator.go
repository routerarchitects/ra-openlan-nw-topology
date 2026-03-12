package owsec

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/client"
	"github.com/router-architects/ra-openlan-nw-topology/adapters/apperrors"
	"github.com/router-architects/ra-openlan-nw-topology/adapters/service_rpc/common"
	servicediscovery "github.com/routerarchitects/ow-common-mods/servicediscovery"
)

const serviceName = "owsec"

type Validator struct {
	deps *common.ServiceRPCBase
}

func NewValidator(deps *common.ServiceRPCBase) *Validator {
	return &Validator{
		deps: deps,
	}
}

func (v *Validator) Validate(rawToken string) error {
	token := strings.TrimSpace(rawToken)

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
		v.deps.Logger.With("service", serviceName, "operation", "validateToken").Error("validation request failed")
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

	reqCtx, cancel := context.WithTimeout(ctx, v.deps.Timeout)
	defer cancel()

	resp, err := v.deps.Client.R().
		SetContext(reqCtx).
		SetTimeout(v.deps.Timeout).
		SetMethod(fiber.MethodGet).
		SetHeader("X-API-KEY", service.Key).
		SetHeader("X-INTERNAL-NAME", v.deps.InternalName).
		SetURL(strings.TrimSuffix(service.PrivateEndPoint, "/") + endpoint).
		Send()
	if err != nil {
		return nil, apperrors.WrapError(apperrors.CodeInternal, "validation request failed", err)
	}

	return resp, nil
}

func (v *Validator) resolveService() (servicediscovery.Instance, error) {
	if v.deps == nil || v.deps.Discovery == nil {
		return servicediscovery.Instance{}, apperrors.WrapError(apperrors.CodeInternal, "service discovery is not configured", nil)
	}

	services := v.deps.Discovery.Store().GetServiceInstances(serviceName)
	if len(services) == 0 {
		return servicediscovery.Instance{}, apperrors.WrapError(apperrors.CodeNotFound, http.StatusText(http.StatusNotFound), nil)
	}

	return services[0], nil
}

package httpclient

import (
	"context"
	"io"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3/client"

	"github.com/router-architects/ra-openlan-nw-topology/adapters/apperrors"
	"github.com/router-architects/ra-openlan-nw-topology/adapters/logger"
	"github.com/router-architects/ra-openlan-nw-topology/internal/models"
)

type OpenAPIRequest struct {
	// store        *store.DiscoveryStore
	client       *client.Client
	timeout      time.Duration
	internalName string
}

type OpenAPIRequestConfig struct {
	Timeout             time.Duration
	InternalServiceName string
}

const (
	defaultRequestTimeout = 3 * time.Second
)

func NewOpenApiRequest(client *client.Client, cfg OpenAPIRequestConfig) *OpenAPIRequest {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultRequestTimeout
	}

	internalName := strings.TrimSpace(cfg.InternalServiceName)
	if internalName == "" {
		internalName = "nw-topology-service"
	}

	return &OpenAPIRequest{
		client:       client,
		timeout:      timeout,
		internalName: internalName,
	}
}

func (v *OpenAPIRequest) Do(ctx context.Context, method string, serviceType string, endPoint string, body io.Reader, services []models.DiscoveryEvent) (*client.Response, error) {
	baseLog := logger.GetLoggerThreadId("SERVER").WithFields(logger.Fields{
		"serviceType": serviceType,
		"method":      method,
		"endpoint":    endPoint,
	})
	baseLog.WithField("discovered_services", len(services)).Trace("discovered services for serviceType %s", serviceType)

	for _, svc := range services {

		fullURL := strings.TrimSuffix(svc.PrivateEndPoint, "/") + endPoint
		log := baseLog.WithField("target", fullURL)

		reqCtx, cancel := context.WithTimeout(ctx, v.timeout)
		defer cancel()

		req := v.client.R().
			SetContext(reqCtx).
			SetTimeout(v.timeout).
			SetMethod(method).
			SetHeader("X-API-KEY", svc.Key).
			SetHeader("X-INTERNAL-NAME", v.internalName).
			SetURL(fullURL)

		if body != nil {
			rawBody, err := io.ReadAll(body)
			if err != nil {
				return nil, apperrors.WrapError(apperrors.CodeInternal, "failed to read body", err)
			}
			req = req.
				SetRawBody(rawBody).
				SetHeader("Content-Type", "application/json")
		}
		start := time.Now()

		resp, err := req.Send()

		if err != nil {
			return nil, apperrors.WrapError(apperrors.CodeUnauthorized, "unauthorized", err)
		}
		log.WithFields(logger.Fields{
			"status":      resp.StatusCode(),
			"duration_ms": time.Since(start).Milliseconds(),
		}).Trace("service request completed for serviceType %s", serviceType)
		return resp, nil
	}
	baseLog.Warnf("no services discovered in store for serviceType %s", serviceType)
	return nil, apperrors.WrapError(apperrors.CodeNotFound, "Not Found", nil)
}

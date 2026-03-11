package httpclient

import (
	"context"
	"crypto/x509"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3/client"
	"github.com/router-architects/ra-openlan-nw-topology/adapters/apperrors"
	servicediscovery "github.com/routerarchitects/ow-common-mods/servicediscovery"
)

type OpenAPIRequest struct {
	// store        *store.DiscoveryStore
	client       *client.Client
	timeout      time.Duration
	internalName string
	logger       *slog.Logger
}

type OpenAPIRequestConfig struct {
	Timeout             time.Duration
	InternalServiceName string
}

const (
	defaultRequestTimeout = 15 * time.Second
)

func NewOpenApiRequest(cfg OpenAPIRequestConfig, logger *slog.Logger, tlsRootCA string) *OpenAPIRequest {
	fiberClient := client.New()
	fiberClient.SetTimeout(5 * time.Second)

	if tlsRootCA != "" {
		pemBytes, err := os.ReadFile(tlsRootCA)
		if err != nil {
			panic(fmt.Sprintf("read TLS root CA %q: %v", tlsRootCA, err))
		}

		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pemBytes) {
			panic(fmt.Sprintf("parse TLS root CA %q: invalid PEM", tlsRootCA))
		}

		fiberClient.TLSConfig().RootCAs = pool
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultRequestTimeout
	}

	internalName := strings.TrimSpace(cfg.InternalServiceName)
	if internalName == "" {
		internalName = "nw-topology-service"
	}

	return &OpenAPIRequest{
		client:       fiberClient,
		timeout:      timeout,
		internalName: internalName,
		logger:       logger,
	}
}

func (v *OpenAPIRequest) Do(method string, serviceType string, endPoint string, body io.Reader, services []servicediscovery.Instance) (*client.Response, error) {
	//TODO : Correct it when i have one instance of service discovery and i can get the service from there instead of passing it as parameter
	for _, svc := range services {

		fullURL := strings.TrimSuffix(svc.PrivateEndPoint, "/") + endPoint

		reqCtx, cancel := context.WithTimeout(context.Background(), v.timeout)
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

		resp, err := req.Send()

		if err != nil {
			return nil, apperrors.WrapError(apperrors.CodeInternal, "unauthorized", err)
		}
		return resp, nil
	}
	return nil, apperrors.WrapError(apperrors.CodeNotFound, "Not Found", nil)
}

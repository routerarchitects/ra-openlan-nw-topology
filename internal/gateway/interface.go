package gateway

import (
	"context"
	"io"

	"github.com/gofiber/fiber/v3/client"
	"github.com/router-architects/ra-openlan-nw-topology/internal/models"
)

type OpenAPIRequestClient interface {
	Do(ctx context.Context, method string, serviceType string, endPoint string, body io.Reader, services []models.DiscoveryEvent) (*client.Response, error)
}

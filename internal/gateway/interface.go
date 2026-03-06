package gateway

import (
	"context"
	"io"

	"github.com/gofiber/fiber/v3/client"
	servicediscovery "github.com/routerarchitects/ow-common-mods/servicediscovery"
)

type OpenAPIRequestClient interface {
	Do(ctx context.Context, method string, serviceType string, endPoint string, body io.Reader, services []servicediscovery.Instance) (*client.Response, error)
}

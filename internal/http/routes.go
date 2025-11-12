package http

import (
	"github.com/router-architects/network-topology-service/internal/http/handlers"
	"github.com/router-architects/network-topology-service/internal/http/middlewares"

	"github.com/gofiber/fiber/v3"
)

func (s *ServerDeps) RegisterRoutes(app *fiber.App, th *handlers.TopologyHandler) {
	v1 := app.Group("/v1", middlewares.APIKeyAuth(s.APIKey, s.TokenValidator))
	v1.Get("/topology", th.GetTopology)
}

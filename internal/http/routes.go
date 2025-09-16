package http

import (
	"github.com/router-architects/network-topology-service/internal/http/handlers"

	"github.com/gofiber/fiber/v3"
)

func (s *ServerDeps) RegisterRoutes(app *fiber.App, th *handlers.TopologyHandler) {
	v1 := app.Group("/v1")
	v1.Get("/topology", th.GetTopology)
}

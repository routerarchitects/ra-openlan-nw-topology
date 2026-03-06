package api

import (
	"github.com/router-architects/ra-openlan-nw-topology/internal/api/handlers"

	"github.com/gofiber/fiber/v3"
)

func (s *Server) RegisterRoutes(app *fiber.App, th *handlers.TopologyHandler) {
	v1 := app.Group("/api/v1", s.AuthMiddleware.TopologyAuth)
	v1.Get("/topology", th.GetTopology)
}

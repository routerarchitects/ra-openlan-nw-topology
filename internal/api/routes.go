package api

import (
	"github.com/router-architects/ra-openlan-nw-topology/internal/api/handlers"
	logger_routes "github.com/routerarchitects/ra-common-mods/logger-routes"

	"github.com/gofiber/fiber/v3"
)

func (s *Server) RegisterRoutes(app *fiber.App, th *handlers.TopologyHandler) {
	v1 := app.Group("/api/v1", s.AuthMiddleware.TopologyAuth)
	v1.Get("/topology", th.GetTopology)
	logger_routes.RegisterFiberRoutes(app.Group("/logger", s.AuthMiddleware.TopologyAuth))
}

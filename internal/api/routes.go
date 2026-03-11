package api

import (
	"github.com/router-architects/ra-openlan-nw-topology/internal/api/handlers"
	logger_routes "github.com/routerarchitects/ra-common-mods/logger-routes"

	"github.com/gofiber/fiber/v3"
)

func (s *Server) RegisterRoutes(publicApp *fiber.App, privateApp *fiber.App, th *handlers.TopologyHandler) {
	publicApp.Get("/livez", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	v1 := publicApp.Group("/api/v1")
	v1.Get("/topology", th.GetTopology)

	privateApp.Get("/livez", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	v2 := privateApp.Group("/api/v1")
	v2.Get("/topology", th.GetTopology)

	logger_routes.RegisterFiberRoutes(publicApp.Group("/logger"))
}

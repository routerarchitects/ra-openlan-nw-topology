package api

import (
	"github.com/router-architects/ra-openlan-nw-topology/internal/api/handlers"

	"github.com/gofiber/fiber/v3"
)

func (s *Server) RegisterRoutes(app *fiber.App, th *handlers.TopologyHandler) {
	app.Get("/livez", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	noAuth := app.Group("/api/v1")
	noAuth.Get("/system", handlers.SystemInfoHandler)
	noAuth.Post("/system", handlers.SystemAPIHandler)
	v1 := app.Group("/api/v1", s.AuthMiddleware.TopologyAuth)
	v1.Get("/topology", th.GetTopology)
}

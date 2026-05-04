package router

import (
	"github.com/gofiber/fiber/v3"
	"github.com/router-architects/ra-openlan-nw-topology/internal/http/handlers"
	subsysteroutes "github.com/routerarchitects/ow-common-mods/fiber/system-routes"
)

type PublicDeps struct {
	AuthHandler fiber.Handler
	Topology    *handlers.TopologyHandler
	Subsystem   subsysteroutes.Config
}

type PrivateDeps struct {
	AuthHandler fiber.Handler
	Topology    *handlers.TopologyHandler
	Subsystem   subsysteroutes.Config
}

func RegisterPublic(app *fiber.App, deps PublicDeps) {
	registerLivenessRoute(app)
	group := app.Group("", deps.AuthHandler)
	registerTopologyRoute(group, deps.Topology)

	subsysteroutes.RegisterRoutes(deps.Subsystem, group)
}

func RegisterPrivate(app *fiber.App, deps PrivateDeps) {
	registerLivenessRoute(app)
	group := app.Group("", deps.AuthHandler)
	registerTopologyRoute(group, deps.Topology)

	subsysteroutes.RegisterRoutes(deps.Subsystem, group)
}

func registerLivenessRoute(app *fiber.App) {
	app.Get("/livez", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
}

func registerTopologyRoute(group fiber.Router, topology *handlers.TopologyHandler) {
	group.Get("/api/v1/topology", topology.GetTopology)
}

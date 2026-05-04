package middleware

import (
	"log/slog"

	"github.com/gofiber/fiber/v3"
	"github.com/routerarchitects/ow-common-mods/fiber/middleware/requestlog"
)

func RegisterRequestLog(app *fiber.App, logger *slog.Logger) {
	app.Use(requestlog.RequestLogger(logger))
}

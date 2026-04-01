package handlers

import (
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/router-architects/ra-openlan-nw-topology/adapters/apperrors"
	"github.com/router-architects/ra-openlan-nw-topology/internal/models"
	"github.com/router-architects/ra-openlan-nw-topology/internal/services"
)

func SystemInfoHandler(c fiber.Ctx) error {

	command := c.Query("command")

	switch strings.ToLower(command) {

	case "info":
		data := services.SystemInfo()
		return c.JSON(data)

	default:
		return fiber.NewError(fiber.StatusBadRequest, "invalid command")
	}
}

func SystemAPIHandler(c fiber.Ctx) error {

	var req struct {
		Command string `json:"command"`
	}

	if err := c.Bind().Body(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid JSON")
	}

	switch strings.ToLower(req.Command) {

	case "getloglevels":
		pairs := services.ListSubsystemLogLevelPairs()
		return c.JSON(fiber.Map{
			"tagList": pairs,
		})

	case "setloglevel":

		var payload models.SetSubsytemPayload
		if err := c.Bind().Body(&payload); err != nil || len(payload.Subsystems) == 0 {
			return writeErrorResponse(c, apperrors.CodeInvalidInput)
		}

		services.SetSubsystemLevel(&payload)

		return c.JSON(fiber.Map{"ok": true})

	case "getloglevelnames":
		logLevelNames := services.GetLogLevelNames()
		return c.JSON(fiber.Map{
			"list": logLevelNames,
		})

	case "getsubsystemnames":
		subSystemNames := services.ListSubsystemNames()
		return c.JSON(fiber.Map{
			"list": subSystemNames,
		})

	default:
		return writeErrorResponse(c, apperrors.CodeInvalidInput)
	}
}

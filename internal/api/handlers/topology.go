package handlers

import (
	"context"
	"fmt"

	"github.com/gofiber/fiber/v3"

	"github.com/router-architects/ra-openlan-nw-topology/adapters/apperrors"
	"github.com/router-architects/ra-openlan-nw-topology/adapters/logger"
	"github.com/router-architects/ra-openlan-nw-topology/internal/models"
)

type TopologyService interface {
	BuildTopology(ctx context.Context, boardID string) (models.Topology, error)
}

type TopologyHandler struct {
	svc TopologyService
}

func NewTopologyHandler(s TopologyService) *TopologyHandler {
	return &TopologyHandler{svc: s}
}

func (h *TopologyHandler) GetTopology(c fiber.Ctx) error {
	log := logger.GetLoggerThreadId("SERVER")
	params := &models.TimepointsQuery{}
	if err := c.Bind().Query(params); err != nil {
		if log != nil {
			log.WithError(err).Warn("failed to bind topology query params")
		}
		return writeErrorResponse(c, apperrors.CodeInvalidInput)
	}

	if params.BoardID == "" {
		if log != nil {
			log.Warn("missing boardId in topology query params")
		}
		return writeErrorResponse(c, apperrors.CodeInvalidInput)
	}

	topo, err := h.svc.BuildTopology(c.Context(), params.BoardID)
	if err != nil {
		appErr, ok := err.(*apperrors.Error)
		if !ok {
			return writeErrorResponse(c, apperrors.CodeInternal)
		}
		if log != nil {
			log.WithError(err).
				WithField("errorCode", appErr.Code).
				Error("topology build failed")
		}
		return writeErrorResponse(c, appErr.Code)
	}
	return c.Status(fiber.StatusOK).JSON(topo)
}

func writeErrorResponse(c fiber.Ctx, code apperrors.ErrorCode) error {
	info := apperrors.GetHTTPErrorInfo(code)
	body := map[string]any{
		"ErrorCode":        info.Status,
		"ErrorDescription": fmt.Sprintf("%d: %s", info.Status, info.Description),
		"ErrorDetails":     c.Method(),
	}
	return c.Status(info.Status).JSON(body)
}

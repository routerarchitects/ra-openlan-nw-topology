package handlers

import (
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/router-architects/ra-openlan-nw-topology/internal/http/response"
	"github.com/router-architects/ra-openlan-nw-topology/internal/models"
	"github.com/routerarchitects/ra-common-mods/apperror"
)

const (
	defaultHistoryDurationSeconds uint64 = 1800
	defaultHistoryJitterSeconds   uint64 = 10
)

func (h *TopologyHandler) GetHistoricalTopology(c fiber.Ctx) error {
	params := &models.HistoricalTopologyQuery{}
	if err := c.Bind().Query(params); err != nil {
		return response.WriteErrorResponse(c, apperror.Wrap(apperror.CodeInvalidInput, "invalid historical topology query parameters", err), apperror.CodeInvalidInput)
	}

	boardID := strings.TrimSpace(params.BoardID)
	if boardID == "" {
		return response.WriteErrorResponse(c, apperror.New(apperror.CodeInvalidInput, "boardId is required"), apperror.CodeInvalidInput)
	}

	if params.WindowInterval == nil {
		return response.WriteErrorResponse(c, apperror.New(apperror.CodeInvalidInput, "windowInterval is required"), apperror.CodeInvalidInput)
	}
	windowInterval := *params.WindowInterval
	if windowInterval == 0 {
		return response.WriteErrorResponse(c, apperror.New(apperror.CodeInvalidInput, "windowInterval must be greater than 0"), apperror.CodeInvalidInput)
	}

	duration := defaultHistoryDurationSeconds
	if params.Duration != nil {
		duration = *params.Duration
		if duration == 0 {
			return response.WriteErrorResponse(c, apperror.New(apperror.CodeInvalidInput, "duration must be greater than 0"), apperror.CodeInvalidInput)
		}
	}

	if windowInterval > duration {
		return response.WriteErrorResponse(c, apperror.New(apperror.CodeInvalidInput, "windowInterval must not exceed duration"), apperror.CodeInvalidInput)
	}

	jitter := defaultHistoryJitterSeconds
	if params.Jitter != nil {
		jitter = *params.Jitter
	}

	history, err := h.svc.BuildHistoricalTopology(c.Context(), boardID, duration, windowInterval, jitter)
	if err != nil {
		return response.WriteErrorResponse(c, err, apperror.CodeInternal)
	}
	return c.Status(fiber.StatusOK).JSON(history)
}

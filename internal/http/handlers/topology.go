package handlers

import (
	"context"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"

	"github.com/router-architects/ra-openlan-nw-topology/internal/http/response"
	"github.com/router-architects/ra-openlan-nw-topology/internal/models"
	"github.com/routerarchitects/ra-common-mods/apperror"
)

const defaultTopologyIntervalSeconds uint64 = 4 * 60

var validate = validator.New()

func init() {
	_ = validate.RegisterValidation("notblank", func(fl validator.FieldLevel) bool {
		return strings.TrimSpace(fl.Field().String()) != ""
	})
}

type TopologyService interface {
	BuildTopology(ctx context.Context, boardID string, timeIntervalSeconds uint64) (*models.Topology, error)
}

type TopologyHandler struct {
	svc TopologyService
}

func NewTopologyHandler(s TopologyService) *TopologyHandler {
	return &TopologyHandler{svc: s}
}

func (h *TopologyHandler) GetTopology(c fiber.Ctx) error {
	params := &models.TimepointsQuery{}
	if err := c.Bind().Query(params); err != nil {
		return response.WriteErrorResponse(c, apperror.Wrap(apperror.CodeInvalidInput, "invalid topology query parameters", err), apperror.CodeInvalidInput)
	}

	if err := validate.Struct(params); err != nil {
		return response.WriteErrorResponse(c, err, apperror.CodeInvalidInput)
	}

	timeIntervalSeconds := defaultTopologyIntervalSeconds
	if params.Interval != nil {
		timeIntervalSeconds = *params.Interval
	}

	boardID := strings.TrimSpace(params.BoardID)

	topo, err := h.svc.BuildTopology(c.Context(), boardID, timeIntervalSeconds)
	if err != nil {
		return response.WriteErrorResponse(c, err, apperror.CodeInternal)
	}
	return c.Status(fiber.StatusOK).JSON(topo)
}

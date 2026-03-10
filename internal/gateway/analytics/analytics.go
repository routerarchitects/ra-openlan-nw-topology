package analytics

import (
	"context"
	"encoding/json"
	"log/slog"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/router-architects/ra-openlan-nw-topology/adapters/apperrors"

	"github.com/router-architects/ra-openlan-nw-topology/internal/gateway"
	"github.com/router-architects/ra-openlan-nw-topology/internal/models"
	servicediscovery "github.com/routerarchitects/ow-common-mods/servicediscovery"
)

type analyticsClient struct {
	store  *servicediscovery.Discovery
	client gateway.OpenAPIRequestClient
	logger *slog.Logger
}

func NewAnalyticsClient(client gateway.OpenAPIRequestClient, discovery *servicediscovery.Discovery, logger *slog.Logger) *analyticsClient {
	return &analyticsClient{
		store:  discovery, // Assuming discovery.Store is accessible and of type *discovery.DiscoveryStore
		client: client,
		logger: logger,
	}
}

const (
	owanalytics = "owanalytics"
)

func (v *analyticsClient) GetTimepoints(ctx context.Context, req models.TimepointRequest) ([]models.TimepointsData, error) {
	fullURL := "/api/v1/board"
	fullURL += "/" + req.BoardID

	fullURL += "/timepoints?"

	if req.FromDate != nil {
		fullURL += "fromDate=" + strconv.FormatUint(*req.FromDate, 10) + "&"
	}
	if req.EndDate != nil {
		fullURL += "endDate=" + strconv.FormatUint(*req.EndDate, 10) + "&"
	}
	if req.MaxRecords != nil {
		fullURL += "maxRecords=" + strconv.Itoa(*req.MaxRecords) + "&"
	}
	if req.StatsOnly {
		fullURL += "statsOnly=true&"
	}
	if req.PointsOnly {
		fullURL += "pointsOnly=true&"
	}
	if req.PointStatsOnly {
		fullURL += "pointStatsOnly=true&"
	}
	if req.Latest {
		fullURL += "LatestPerDevice=true"
	}

	start := time.Now()

	services := v.store.Store().GetServiceInstances(owanalytics)

	resp, err := v.client.Do(ctx, fiber.MethodGet, "owanalytics", fullURL, nil, services)

	if err != nil {
		return nil, apperrors.WrapError(apperrors.CodeInternal, "failed to get timepoints", err)
	}

	if resp.StatusCode() == fiber.StatusNotFound {
		v.logger.With("status", resp.StatusCode()).Error("timepoints not found")
		info := apperrors.GetHTTPErrorInfo(apperrors.CodeNotFound)
		return nil, apperrors.WrapError(apperrors.CodeNotFound, info.Description, nil)
	}

	if resp.StatusCode() != fiber.StatusOK {
		return nil, apperrors.WrapError(apperrors.CodeInternal, "failed to get timepoints: non-200 response", nil)
	}

	type timepointResponse struct {
		Points [][]models.TimepointsData `json:"points"`
	}
	var tpResp timepointResponse
	if err := json.Unmarshal(resp.Body(), &tpResp); err != nil {
		return nil, apperrors.WrapError(apperrors.CodeInternal, "failed to parse timepoints response", err)
	}

	var timepoints []models.TimepointsData
	for _, bucket := range tpResp.Points {
		if len(bucket) == 0 {
			continue
		}
		timepoints = append(timepoints, bucket...)
	}

	v.logger.With("records", len(timepoints), "status", resp.StatusCode(), "duration_ms", time.Since(start).Milliseconds()).Info("received timepoints response")

	return timepoints, nil

}

func (v *analyticsClient) GetDeviceInfo(ctx context.Context, boardId string) ([]models.DeviceInfo, error) {
	// Placeholder for future implementation
	fullURL := "/api/v1/board/" + boardId + "/devices"

	services := v.store.Store().GetServiceInstances(owanalytics)

	resp, err := v.client.Do(ctx, fiber.MethodGet, "owanalytics", fullURL, nil, services)

	if err != nil {
		return []models.DeviceInfo{}, apperrors.WrapError(apperrors.CodeInternal, "failed to get device info", err)
	}

	if resp.StatusCode() == fiber.StatusNotFound {
		info := apperrors.GetHTTPErrorInfo(apperrors.CodeNotFound)
		return []models.DeviceInfo{}, apperrors.WrapError(apperrors.CodeNotFound, info.Description, nil)
	}

	if resp.StatusCode() != fiber.StatusOK {
		return []models.DeviceInfo{}, apperrors.WrapError(apperrors.CodeInternal, "failed to get device info: non-200 response", nil)
	}

	type DeviceInfoResponse struct {
		Devices []models.DeviceInfo `json:"devices"`
	}

	var deviceInfo DeviceInfoResponse
	if err := json.Unmarshal(resp.Body(), &deviceInfo); err != nil {
		return []models.DeviceInfo{}, apperrors.WrapError(apperrors.CodeInternal, "failed to parse device info response", err)
	}

	return deviceInfo.Devices, nil
}

package analytics

import (
	"context"
	"encoding/json"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/router-architects/ra-openlan-nw-topology/adapters/apperrors"
	"github.com/router-architects/ra-openlan-nw-topology/adapters/logger"
	"github.com/router-architects/ra-openlan-nw-topology/internal/gateway"
	"github.com/router-architects/ra-openlan-nw-topology/internal/models"
	"github.com/router-architects/ra-openlan-nw-topology/internal/services/discovery"
)

type analyticsClient struct {
	store  *discovery.DiscoveryStore
	client gateway.OpenAPIRequestClient
}

func NewAnalyticsClient(client gateway.OpenAPIRequestClient, store *discovery.DiscoveryStore) *analyticsClient {
	return &analyticsClient{
		store:  store,
		client: client,
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
		fullURL += "pointStatsOnly=true"
	}
	if req.Latest {
		fullURL += "LatestPerDevice=true"
	}

	logFields := logger.Fields{
		"boardId":        req.BoardID,
		"statsOnly":      req.StatsOnly,
		"pointsOnly":     req.PointsOnly,
		"pointStatsOnly": req.PointStatsOnly,
	}
	if req.FromDate != nil {
		logFields["fromDate"] = *req.FromDate
	}
	if req.EndDate != nil {
		logFields["endDate"] = *req.EndDate
	}
	if req.MaxRecords != nil {
		logFields["maxRecords"] = *req.MaxRecords
	}
	log := logger.GetLoggerThreadId("SERVER").WithFields(logFields)
	start := time.Now()

	services := v.store.GetServices(owanalytics)

	resp, err := v.client.Do(ctx, fiber.MethodGet, "owanalytics", fullURL, nil, services)

	if err != nil {
		return nil, apperrors.WrapError(apperrors.CodeInternal, "failed to get timepoints", err)
	}

	if resp.StatusCode() == fiber.StatusNotFound {
		log.WithField("status", resp.StatusCode()).Error("timepoints not found")
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
	// Keep the timepoints ordered with the most recent timestamp first.
	sort.Slice(timepoints, func(i, j int) bool {
		return timepoints[i].Timestamp > timepoints[j].Timestamp
	})

	log.WithFields(logger.Fields{
		"records":     len(timepoints),
		"status":      resp.StatusCode(),
		"duration_ms": time.Since(start).Milliseconds(),
	}).Trace("received timepoints response")

	return timepoints, nil

}

func (v *analyticsClient) GetDeviceInfo(ctx context.Context, boardId string) ([]models.DeviceInfo, error) {
	// Placeholder for future implementation
	fullURL := "/api/v1/board/" + boardId + "/devices"

	services := v.store.GetServices(owanalytics)

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

func (v *analyticsClient) GetWifiClientHistoryMACs(ctx context.Context, boardId string, limit, offset int) ([]string, error) {

	fullURL := "/api/v1/wifiClientHistory" +
		"?macsOnly=true" +
		"&boardId=" + url.QueryEscape(strings.TrimSpace(boardId)) +
		"&limit=" + strconv.Itoa(limit) +
		"&offset=" + strconv.Itoa(offset)

	log := logger.GetLoggerThreadId("SERVER").WithFields(logger.Fields{
		"boardId":  boardId,
		"limit":  limit,
		"offset": offset,
	})

	start := time.Now()

	services := v.store.GetServices(owanalytics)

	resp, err := v.client.Do(ctx, fiber.MethodGet, owanalytics, fullURL, nil, services)
	if resp != nil {
		defer resp.Close()
	}

	if err != nil {
		return nil, apperrors.WrapError(apperrors.CodeInternal, "failed to get wifiClientHistory", err)
	}

	if resp.StatusCode() == fiber.StatusNotFound {
		info := apperrors.GetHTTPErrorInfo(apperrors.CodeNotFound)
		return nil, apperrors.WrapError(apperrors.CodeNotFound, info.Description, nil)
	}

	if resp.StatusCode() != fiber.StatusOK {
		return nil, apperrors.WrapError(apperrors.CodeInternal, "failed to get wifiClientHistory: non-200 response", nil)
	}

	type wifiClientHistoryResponse struct {
		Entries []string `json:"entries"`
	}

	var out wifiClientHistoryResponse
	if err := json.Unmarshal(resp.Body(), &out); err != nil {
		return nil, apperrors.WrapError(apperrors.CodeInternal, "failed to parse wifiClientHistory response", err)
	}

	log.WithFields(logger.Fields{
		"entries":     len(out.Entries),
		"status":      resp.StatusCode(),
		"duration_ms": time.Since(start).Milliseconds(),
	}).Trace("received wifiClientHistory response")

	return out.Entries, nil
}

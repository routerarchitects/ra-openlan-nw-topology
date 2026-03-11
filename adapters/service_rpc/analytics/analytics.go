package analytics

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/client"
	"github.com/router-architects/ra-openlan-nw-topology/adapters/apperrors"
	"github.com/router-architects/ra-openlan-nw-topology/adapters/service_rpc/common"
	"github.com/router-architects/ra-openlan-nw-topology/internal/models"
	servicediscovery "github.com/routerarchitects/ow-common-mods/servicediscovery"
)

const serviceName = "owanalytics"

type Client struct {
	discovery    *servicediscovery.Discovery
	client       *client.Client
	timeout      time.Duration
	internalName string
	logger       *slog.Logger
}

func NewClient(
	discovery *servicediscovery.Discovery,
	logger *slog.Logger,
	tlsRootCA string,
	timeout time.Duration,
	internalName string,
) *Client {
	timeout = common.NormalizeTimeout(timeout)

	return &Client{
		discovery:    discovery,
		client:       common.NewFiberClient(timeout, tlsRootCA),
		timeout:      timeout,
		internalName: common.NormalizeInternalName(internalName),
		logger:       logger,
	}
}

func (v *Client) GetTimepoints(req models.TimepointRequest) ([]models.TimepointsData, error) {
	fullURL := "/api/v1/board/" + req.BoardID + "/timepoints?"

	if req.FromDate != nil {
		fullURL += "fromDate=" + strconv.FormatUint(*req.FromDate, 10) + "&"
	}
	if req.EndDate != nil {
		fullURL += "endDate=" + strconv.FormatUint(*req.EndDate, 10) + "&"
	}
	if req.MaxRecords != nil {
		fullURL += "maxRecords=" + strconv.Itoa(*req.MaxRecords) + "&"
	}
	fullURL += "statsOnly=true&pointsOnly=true&pointStatsOnly=true&LatestPerDevice=true"

	start := time.Now()
	resp, err := v.send(context.Background(), fiber.MethodGet, fullURL, nil)
	if err != nil {
		return nil, apperrors.WrapError(apperrors.CodeInternal, "failed to get timepoints", err)
	}
	defer resp.Close()

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

	v.logger.With(
		"records", len(timepoints),
		"status", resp.StatusCode(),
		"duration_ms", time.Since(start).Milliseconds(),
	).Debug("received timepoints response")

	return timepoints, nil
}

func (v *Client) GetDeviceInfo(boardID string) ([]models.DeviceInfo, error) {
	resp, err := v.send(context.Background(), fiber.MethodGet, "/api/v1/board/"+boardID+"/devices", nil)
	if err != nil {
		return []models.DeviceInfo{}, apperrors.WrapError(apperrors.CodeInternal, "failed to get device info", err)
	}
	defer resp.Close()

	if resp.StatusCode() == fiber.StatusNotFound {
		info := apperrors.GetHTTPErrorInfo(apperrors.CodeNotFound)
		return []models.DeviceInfo{}, apperrors.WrapError(apperrors.CodeNotFound, info.Description, nil)
	}

	if resp.StatusCode() != fiber.StatusOK {
		return []models.DeviceInfo{}, apperrors.WrapError(apperrors.CodeInternal, "failed to get device info: non-200 response", nil)
	}

	type deviceInfoResponse struct {
		Devices []models.DeviceInfo `json:"devices"`
	}

	var payload deviceInfoResponse
	if err := json.Unmarshal(resp.Body(), &payload); err != nil {
		return []models.DeviceInfo{}, apperrors.WrapError(apperrors.CodeInternal, "failed to parse device info response", err)
	}

	return payload.Devices, nil
}

func (v *Client) send(ctx context.Context, method string, endpoint string, body io.Reader) (*client.Response, error) {
	service, err := v.resolveService()
	if err != nil {
		return nil, err
	}

	reqCtx, cancel := context.WithTimeout(ctx, v.timeout)
	defer cancel()

	req := v.client.R().
		SetContext(reqCtx).
		SetTimeout(v.timeout).
		SetMethod(method).
		SetHeader("X-API-KEY", service.Key).
		SetHeader("X-INTERNAL-NAME", v.internalName).
		SetURL(strings.TrimSuffix(service.PrivateEndPoint, "/") + endpoint)

	if body != nil {
		rawBody, err := io.ReadAll(body)
		if err != nil {
			return nil, apperrors.WrapError(apperrors.CodeInternal, "failed to read body", err)
		}
		req = req.SetRawBody(rawBody).SetHeader(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	}

	resp, err := req.Send()
	if err != nil {
		return nil, apperrors.WrapError(apperrors.CodeInternal, "analytics request failed", err)
	}

	return resp, nil
}

func (v *Client) resolveService() (servicediscovery.Instance, error) {
	if v.discovery == nil {
		return servicediscovery.Instance{}, apperrors.WrapError(apperrors.CodeInternal, "service discovery is not configured", nil)
	}

	services := v.discovery.Store().GetServiceInstances(serviceName)
	if len(services) == 0 {
		return servicediscovery.Instance{}, apperrors.WrapError(apperrors.CodeNotFound, http.StatusText(http.StatusNotFound), nil)
	}

	return services[0], nil
}

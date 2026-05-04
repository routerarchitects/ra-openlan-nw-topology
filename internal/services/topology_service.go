package services

import (
	"context"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/router-architects/ra-openlan-nw-topology/internal/models"
	"github.com/routerarchitects/ow-common-mods/servicerpc/analytics"
	"github.com/routerarchitects/ra-common-mods/logger"
)

// AnalyticsClientInterface is the service boundary for all topology input data.
// The topology builder keeps transformation rules local and treats analytics as
// the source of raw timepoints, deviceinfo, and historical client MACs.
type AnalyticsClientInterface interface {
	GetTimepoints(ctx context.Context, req analytics.TimepointRequest) ([]analytics.TimepointsData, error)
	GetDeviceInfo(ctx context.Context, boardId string) ([]analytics.DeviceInfo, error)
	GetWifiClientHistoryMACs(ctx context.Context, boardId string, limit, offset int) ([]string, error)
}

// when the handler does not receive a time query parameter. last four minutes is window to capture recent timepoints data

type topologyService struct {
	client AnalyticsClientInterface
	logger *slog.Logger
}

type topologyInputs struct {
	rows             []analytics.TimepointsData
	deviceInfoStatus map[string]bool
	macs             []string
	nowUnix          int64
}

type bssidOwnership struct {
	bssidOwner map[string]string
}

type topologyBuildResult struct {
	devMap       map[string]*models.Device
	meshEdgesMap map[string]models.MeshEdge
}

func NewTopologyService(client AnalyticsClientInterface) *topologyService {
	logger := logger.Subsystem("topology-service")
	return &topologyService{client: client, logger: logger}
}

func (s *topologyService) BuildTopology(ctx context.Context, boardID string, timeIntervalSeconds uint64) (*models.Topology, error) {

	inputs, err := s.fetchTopologyInputs(ctx, boardID, timeIntervalSeconds)
	if err != nil {
		return nil, err
	}

	present := s.buildPresentSet(inputs.rows)
	historical := s.buildHistoricalDevices(inputs.macs, present)
	s.logger.Debug(
		"topology inputs prepared",
		"boardID", boardID,
		"timepointRows", len(inputs.rows),
		"deviceInfoCount", len(inputs.deviceInfoStatus),
		"historicalDevices", len(historical),
	)

	if len(inputs.rows) == 0 {
		// No timepoints means there is no recent data of device timepoints. We still return
		// devices information so the caller can know "board has devices"
		s.logger.Info("no timepoint rows; returning empty topology")
		devs := s.buildEmptyDevicesFromDeviceInfo(inputs.deviceInfoStatus)
		out := s.buildTopologyResponse(boardID, inputs.nowUnix, devs, historical, []models.MeshEdge{})
		return &out, nil
	}

	ownership := s.buildBSSIDOwnership(inputs.rows, inputs.deviceInfoStatus)
	buildResult := s.buildDevicesAndMeshEdges(inputs.rows, inputs.deviceInfoStatus, ownership)
	devs := s.buildDevicesFromDevMapAndDeviceInfo(buildResult.devMap, inputs.deviceInfoStatus)
	meshEdges := getEdgesSliceFromMap(buildResult.meshEdgesMap)

	s.sortTopology(devs, meshEdges)

	out := s.buildTopologyResponse(boardID, inputs.nowUnix, devs, historical, meshEdges)
	s.logger.Info("topology built successfully")
	return &out, nil
}

func (s *topologyService) fetchTopologyInputs(ctx context.Context, boardID string, timeIntervalSeconds uint64) (*topologyInputs, error) {
	nowUnix := time.Now().Unix()

	endDate := uint64(nowUnix)
	fromDate := endDate - timeIntervalSeconds
	// fromDate is time earlier than endDate by timeIntervalSeconds
	// so the analytics request will return timepoints within that window.
	maxRecords := 1000

	rows, err := s.client.GetTimepoints(ctx, analytics.TimepointRequest{
		BoardID:    boardID,
		FromDate:   fromDate,
		EndDate:    endDate,
		MaxRecords: maxRecords,
	})
	if err != nil {
		return nil, err
	}
	deviceInfo, err := s.client.GetDeviceInfo(ctx, boardID)
	if err != nil {
		return nil, err
	}

	deviceInfoStatus := make(map[string]bool, len(deviceInfo))
	for _, di := range deviceInfo {
		deviceInfoStatus[di.SerialNumber] = di.Connected
	}

	macs, err := s.client.GetWifiClientHistoryMACs(ctx, boardID, 500, 0)
	if err != nil {
		s.logger.Warn("failed to fetch wifi client history", "err", err)
	} else {
		s.logger.Debug("wifi client macs fetched", "boardID", boardID, "count", len(macs))
	}

	return &topologyInputs{
		rows:             rows,
		deviceInfoStatus: deviceInfoStatus,
		macs:             macs,
		nowUnix:          nowUnix,
	}, nil
}

func (s *topologyService) buildPresentSet(rows []analytics.TimepointsData) map[string]struct{} {
	present := make(map[string]struct{}, 1024)

	for _, r := range rows {
		for _, ssidData := range r.SSIDData {
			bssid := normMAC(ssidData.BSSID)
			if bssid != "" {
				present[bssid] = struct{}{}
			}

			for _, association := range ssidData.Associations {
				stationMAC := normMAC(association.Station)
				if stationMAC != "" {
					// A normalized station MAC in current timepoints is considered
					// present, so it will be excluded from historicalDevices.
					present[stationMAC] = struct{}{}
				}
			}
		}

		serialMAC := normMAC(r.Serial)
		if serialMAC != "" {
			present[serialMAC] = struct{}{}
		}
	}

	return present
}

// buildHistoricalDevices returns historical client MACs that are not visible in
// the current timepoint window.
func (s *topologyService) buildHistoricalDevices(macs []string, present map[string]struct{}) []string {
	historical := make([]string, 0, len(macs))

	for _, rawMAC := range macs {
		normalizedMAC := normMAC(rawMAC)
		if normalizedMAC == "" {
			continue
		}
		if _, ok := present[normalizedMAC]; ok {
			// A MAC that is already visible in the current topology is not
			// historical for this response, so exclude it from historicalDevices.
			continue
		}

		historical = append(historical, normalizedMAC)
	}

	return dedupeList(historical)
}

// buildBSSIDOwnership maps each normalized BSSID to the serial of the device
// that advertised it. Mesh associations later use this map to turn a station
// MAC into a device-to-device edge.
func (s *topologyService) buildBSSIDOwnership(rows []analytics.TimepointsData, deviceInfoStatus map[string]bool) bssidOwnership {
	bssidOwner := make(map[string]string)

	for _, r := range rows {
		serial := resolveSerial(r)
		if serial == "" {
			continue
		}
		if di, ok := deviceInfoStatus[serial]; !ok || !di {
			continue
		}

		for _, ssidData := range r.SSIDData {
			bssid := normMAC(ssidData.BSSID)
			if bssid == "" {
				continue
			}
			bssidOwner[bssid] = serial
		}
	}

	return bssidOwnership{
		bssidOwner: bssidOwner,
	}
}

func (s *topologyService) buildDevicesAndMeshEdges(
	rows []analytics.TimepointsData,
	deviceInfoStatus map[string]bool,
	ownership bssidOwnership,
) topologyBuildResult {
	// knownStation prevents the same Wi-Fi client from being attached to
	// multiple AP Interfaces in one topology response.
	knownStation := make(map[string]bool)
	// devMap is keyed by serial so duplicate timepoint rows collapse into one
	// output node.
	devMap := make(map[string]*models.Device, len(rows))
	// meshEdgeSet is keyed by relationship identity so repeated mesh
	// associations do not create duplicate mesh edges.
	meshEdgeSet := make(map[string]models.MeshEdge)

	for _, r := range rows {
		serial := resolveSerial(r)
		if serial == "" {
			// Rows without a serial cannot be attached to a topology node.
			continue
		}
		if _, ok := devMap[serial]; ok {
			// Keep the first row for a serial in this response. The analytics
			// request is expected to return latest-per-device rows, so duplicates
			// are treated as redundant input.
			continue
		}

		// If device info for this serial does not exist, treat the row as outside
		// the requested board. This prevents stale or cross-board timepoints from
		// creating nodes that device information does not know about.
		_, found := deviceInfoStatus[serial]
		if !found {
			continue
		}

		if deviceInfoStatus[serial] == false {
			// Disconnected devices are still returned as nodes, but their
			// timepoint interfaces are excluded because disconnected devices should not
			// contribute live AP, mesh, or client state.
			devMap[serial] = &models.Device{
				Serial:    serial,
				Connected: false,
				APs:       []models.Iface{},
				Mesh:      []models.Iface{},
			}
			continue
		}

		dev := &models.Device{
			Uptime:    r.DeviceInfo.Uptime,
			Serial:    serial,
			Connected: deviceInfoStatus[serial],
			APs:       []models.Iface{},
			Mesh:      []models.Iface{},
		}

		for _, ssidData := range r.SSIDData {
			Iface, ok := buildBaseIface(ssidData)
			if !ok {
				// A Interface without a usable BSSID cannot be referenced by clients or
				// mesh peers, so exclude it from the topology.
				continue
			}

			clients := s.buildIfaceClientsAndMeshEdges(
				serial,
				ssidData,
				Iface.Mode,
				ownership,
				knownStation,
				meshEdgeSet,
			)

			if len(clients) > 0 {
				Iface.Clients = clients
			}

			switch Iface.Mode {
			case "ap":
				dev.APs = append(dev.APs, Iface)
			case "mesh":
				dev.Mesh = append(dev.Mesh, Iface)
			}
		}

		sortFaces(dev)
		devMap[serial] = dev
	}

	return topologyBuildResult{
		devMap:       devMap,
		meshEdgesMap: meshEdgeSet,
	}
}

func (s *topologyService) buildIfaceClientsAndMeshEdges(
	serial string,
	ssidData analytics.SSIDData,
	mode string,
	ownership bssidOwnership,
	knownStation map[string]bool,
	meshEdgeSet map[string]models.MeshEdge,
) []models.IfaceClient {
	var clients []models.IfaceClient

	for _, association := range ssidData.Associations {
		stationMAC := normMAC(association.Station)
		if stationMAC == "" {
			continue
		}

		switch mode {
		case "ap":
			if knownStation[stationMAC] {
				// A client may appear under multiple AP Interfaces in raw data. Keep the
				// first occurrence so the same client is not duplicated in the
				// topology response.
				continue
			}
			if _, ok := ownership.bssidOwner[stationMAC]; ok {
				// If the station MAC is also a known BSSID, it represents another
				// network device rather than an end-client. Exclude it from AP
				// clients; mesh handling is responsible for device-to-device links.
				continue
			}

			fingerprint := s.extractFingerprint(stationMAC, association.Fingerprint)
			clients = append(clients, models.IfaceClient{
				Station:       stationMAC,
				RSSI:          association.RSSI,
				Connected:     association.Connected,
				Inactive:      association.Inactive,
				RxRateBitrate: association.RxRate.Bitrate,
				TxRateBitrate: association.TxRate.Bitrate,
				RxRateChwidth: association.RxRate.Chwidth,
				Fingerprint:   fingerprint,
			})
			knownStation[stationMAC] = true

		case "mesh":
			toSerial, isBSSID := ownership.bssidOwner[stationMAC]
			if !isBSSID {
				// Mesh Interfaces only keep associations that point at a known device
				// BSSID. Other stations are excluded because they are clients, not
				// topology edges.
				s.logger.Debug("mesh association skipped because station is not a known bssid", "serial", serial, "ssid", ssidData.SSID, "station", stationMAC)
				continue
			}

			clients = append(clients, models.IfaceClient{
				Station:       stationMAC,
				RSSI:          association.RSSI,
				Connected:     association.Connected,
				Inactive:      association.Inactive,
				RxRateBitrate: association.RxRate.Bitrate,
				TxRateBitrate: association.TxRate.Bitrate,
				RxRateChwidth: association.RxRate.Chwidth,
			})

			if toSerial == "" {
				continue
			}

			key := serial + "|" + toSerial + "|" + ssidData.BSSID
			if _, seen := meshEdgeSet[key]; seen {
				// Multiple samples can describe the same mesh relationship. Keep a
				// single edge so topology consumers do not render duplicates.
				continue
			}

			meshEdgeSet[key] = models.MeshEdge{
				From:    serial,
				To:      toSerial,
				SSID:    ssidData.SSID,
				Band:    strconv.Itoa(ssidData.Band),
				Channel: ssidData.Channel,
			}
		}
	}

	return clients
}

func (s *topologyService) buildDevicesFromDevMapAndDeviceInfo(
	devMap map[string]*models.Device,
	deviceInfoStatus map[string]bool,
) []models.Device {
	devs := make([]models.Device, 0, len(devMap)+len(deviceInfoStatus))

	for _, d := range devMap {
		devs = append(devs, *d)
	}

	for serial, connected := range deviceInfoStatus {
		if _, exists := devMap[serial]; !exists {
			// devices missing from the timepoint window are not dropped.
			// Return them as empty nodes so disconnected or quiet devices still
			// appear in the board topology.
			devs = append(devs, models.Device{
				Uptime:    0,
				Serial:    serial,
				Connected: connected,
				APs:       []models.Iface{},
				Mesh:      []models.Iface{},
			})
		}
	}

	return devs
}

func (s *topologyService) sortTopology(devs []models.Device, meshEdges []models.MeshEdge) {
	// Device and edge slices are built from maps, whose iteration order is not
	// stable in Go. Sort them so API responses and tests are deterministic.
	sort.Slice(devs, func(i, j int) bool {
		return devs[i].Serial < devs[j].Serial
	})

	sort.Slice(meshEdges, func(i, j int) bool {
		if meshEdges[i].From == meshEdges[j].From {
			if meshEdges[i].To == meshEdges[j].To {
				if meshEdges[i].SSID == meshEdges[j].SSID {
					if meshEdges[i].Band == meshEdges[j].Band {
						return meshEdges[i].Channel < meshEdges[j].Channel
					}
					return meshEdges[i].Band < meshEdges[j].Band
				}
				return meshEdges[i].SSID < meshEdges[j].SSID
			}
			return meshEdges[i].To < meshEdges[j].To
		}
		return meshEdges[i].From < meshEdges[j].From
	})
}

func (s *topologyService) buildTopologyResponse(
	boardID string,
	nowUnix int64,
	devs []models.Device,
	historical []string,
	meshEdges []models.MeshEdge,
) models.Topology {
	return models.Topology{
		BoardID:           boardID,
		Timestamp:         time.Unix(nowUnix, 0).UTC().Format(time.RFC3339),
		Nodes:             devs,
		HistoricalDevices: historical,
		Edges:             models.TopoEdges{Wired: []any{}, Mesh: meshEdges},
		External:          []any{},
	}
}

func (s *topologyService) buildEmptyDevicesFromDeviceInfo(deviceInfoStatus map[string]bool) []models.Device {
	devs := make([]models.Device, 0, len(deviceInfoStatus))
	for serial, connected := range deviceInfoStatus {
		// With no timepoints, device information is the only source of truth. Every
		// device is returned with empty Iface lists.
		devs = append(devs, models.Device{
			Uptime:    0,
			Serial:    serial,
			Connected: connected,
			APs:       []models.Iface{},
			Mesh:      []models.Iface{},
		})
	}
	sort.Slice(devs, func(i, j int) bool {
		return devs[i].Serial < devs[j].Serial
	})
	return devs
}

func buildBaseIface(ssidData analytics.SSIDData) (models.Iface, bool) {
	bssid := normMAC(ssidData.BSSID)
	if bssid == "" {
		// Interfaces without a normalized BSSID are excluded by returning false.
		// BSSID is the stable identifier used for ownership and mesh matching.
		return models.Iface{}, false
	}

	mode := strings.ToLower(strings.TrimSpace(ssidData.Mode))

	Iface := models.Iface{
		BSSID:   bssid,
		SSID:    ssidData.SSID,
		Band:    strconv.Itoa(ssidData.Band),
		Channel: ssidData.Channel,
		Mode:    mode,
		Clients: nil,
	}
	return Iface, true
}

func sortFaces(dev *models.Device) {
	// Sort AP and mesh Interfaces separately so each device is stable regardless of
	// the order analytics returned SSID records.
	sort.Slice(dev.APs, func(i, j int) bool {
		if dev.APs[i].Band == dev.APs[j].Band {
			return dev.APs[i].BSSID < dev.APs[j].BSSID
		}
		return dev.APs[i].Band < dev.APs[j].Band
	})

	sort.Slice(dev.Mesh, func(i, j int) bool {
		if dev.Mesh[i].Band == dev.Mesh[j].Band {
			return dev.Mesh[i].BSSID < dev.Mesh[j].BSSID
		}
		return dev.Mesh[i].Band < dev.Mesh[j].Band
	})
}

func getEdgesSliceFromMap(meshEdgeSet map[string]models.MeshEdge) []models.MeshEdge {
	// The set removes duplicate edges while building. This converts it back to a
	// slice; sortTopology handles deterministic ordering afterward.
	meshEdges := make([]models.MeshEdge, 0, len(meshEdgeSet))
	for _, e := range meshEdgeSet {
		meshEdges = append(meshEdges, e)
	}
	return meshEdges
}

func resolveSerial(r analytics.TimepointsData) string {
	// Serial is required as the topology node key. Prefer the nested deviceInfo
	// serial because it is the inventory-style identifier used by deviceInfoStatus.
	if serial := strings.TrimSpace(r.DeviceInfo.SerialNumber); serial != "" {
		return serial
	}
	// Fall back to the top-level serial for payloads that do not populate
	// DeviceInfo.SerialNumber.
	return strings.TrimSpace(r.Serial)
}

func (s *topologyService) extractFingerprint(stationMAC string, fingerprint map[string]any) string {
	if fingerprint == nil {
		// No fingerprint metadata was provided, so leave the field empty.
		s.logger.Debug("fingerprint absent", "station", stationMAC)
		return ""
	}
	// Prefer the most user-recognizable label first, then fall back to broader
	// vendor or OS information. "unknown" is returned only when fingerprint data
	// exists but none of the expected fields are populated.
	if v, ok := fingerprint["device_name"].(string); ok && v != "" {
		return v
	}
	if v, ok := fingerprint["vendor"].(string); ok && v != "" {
		return v
	}
	if v, ok := fingerprint["os"].(string); ok && v != "" {
		return v
	}
	return "unknown"
}

func normMAC(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		// Empty identifiers are excluded by callers because they cannot be matched.
		return ""
	}
	if strings.Contains(s, ":") {
		// Already colon-separated; callers rely on the normalized lower-case form.
		return s
	}
	if len(s) != 12 {
		// exact len 12 for a MAC address in compact form, so treat it as invalid.
		return ""
	}
	return s[0:2] + ":" + s[2:4] + ":" +
		s[4:6] + ":" + s[6:8] + ":" +
		s[8:10] + ":" + s[10:12]
}

func dedupeList(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, v := range in {
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			// Keep the first occurrence to preserve source order while removing duplicates.
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

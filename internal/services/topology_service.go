package services

import (
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/router-architects/ra-openlan-nw-topology/internal/models"
)

// gateway interface
type AnalyticsClientInterface interface {
	GetTimepoints(req models.TimepointRequest) ([]models.TimepointsData, error)
	GetDeviceInfo(boardId string) ([]models.DeviceInfo, error)
}

type topologyService struct {
	client AnalyticsClientInterface
	logger *slog.Logger
}

func NewTopologyService(client AnalyticsClientInterface, logger *slog.Logger) *topologyService {
	return &topologyService{client: client, logger: logger}
}

func (s *topologyService) BuildTopology(boardID string) (models.Topology, error) {

	var fromDate uint64
	var endDate uint64
	fromDate = uint64(time.Now().Add(-4 * time.Minute).Unix())
	endDate = uint64(time.Now().Unix())

	nowUnix := time.Now().Unix()
	maxRecords := 1000

	rows, err := s.client.GetTimepoints(models.TimepointRequest{
		BoardID:    boardID,
		FromDate:   fromDate,
		EndDate:    endDate,
		MaxRecords: maxRecords,
	})
	if err != nil {
		s.logger.Error("fetch timepoints failed")
		return models.Topology{}, err
	}

	deviceInfo, err := s.client.GetDeviceInfo(boardID)
	if err != nil {
		s.logger.Error("fetch device info failed")
		return models.Topology{}, err
	}
	deviceInfoStatus := make(map[string]bool)
	for _, di := range deviceInfo {
		deviceInfoStatus[di.SerialNumber] = di.Connected
	}

	// 1) No rows -> empty topology
	if len(rows) == 0 {

		s.logger.Info("no timepoint rows; returning empty topology")
		dev := []models.Device{}
		for _, m := range deviceInfo {
			dev = append(dev, models.Device{
				Uptime:    0,
				Serial:    m.SerialNumber,
				Connected: m.Connected,
				APs:       []models.Face{},
				Mesh:      []models.Face{},
			})
		}
		return models.Topology{
			BoardID:   boardID,
			Timestamp: time.Unix(nowUnix, 0).UTC().Format(time.RFC3339),
			Nodes:     dev,
			Edges:     models.TopoEdges{Wired: []any{}, Mesh: []models.MeshEdge{}},
			External:  []any{},
		}, nil
	}

	// Load IST once
	ist, tzErr := time.LoadLocation("Asia/Kolkata")
	if tzErr != nil {
		ist = time.FixedZone("IST", 5*60*60+30*60)
	}

	// 2) Pass 1: build knownBSSID + bssidOwner
	knownBSSID := make(map[string]struct{})
	bssidOwner := make(map[string]string) // bssid -> serial

	for _, r := range rows {
		serial := strings.TrimSpace(r.Serial)
		if sn := strings.TrimSpace(r.DeviceInfo.SerialNumber); sn != "" {
			serial = sn
		}
		serial = strings.TrimSpace(serial)
		if serial == "" {
			continue
		}

		for _, f := range r.SSIDData {
			b := normMAC(f.BSSID)
			if b == "" {
				continue
			}
			knownBSSID[b] = struct{}{}
			// one row per serial => owner is unambiguous
			bssidOwner[b] = serial
		}
	}

	knownStation := make(map[string]bool)

	// 3) Pass 2: build devices + faces + clients + mesh edges
	devMap := make(map[string]*models.Device, len(rows))
	meshEdgeSet := make(map[string]models.MeshEdge) // dedupe: from|to|ssid|band|channel

	for _, r := range rows {
		serial := strings.TrimSpace(r.Serial)
		if sn := strings.TrimSpace(r.DeviceInfo.SerialNumber); sn != "" {
			serial = sn
		}
		serial = strings.TrimSpace(serial)
		if serial == "" {
			continue
		}

		dev := &models.Device{
			Uptime:    r.DeviceInfo.Uptime,
			Serial:    serial,
			Connected: deviceInfoStatus[serial],
			APs:       []models.Face{},
			Mesh:      []models.Face{},
		}

		if deviceInfoStatus[serial] == false {
			// skip offline devices' faces
			offlineDev := &models.Device{
				Serial:    serial,
				Connected: false,
				APs:       []models.Face{},
				Mesh:      []models.Face{},
			}
			devMap[serial] = offlineDev
			continue
		}

		rowTS := time.Unix(r.Timestamp, 0).In(ist).Format(time.RFC3339)

		for _, f := range r.SSIDData {
			b := normMAC(f.BSSID)
			if b == "" {
				continue
			}

			mode := strings.ToLower(strings.TrimSpace(f.Mode))

			face := models.Face{
				BSSID:     b,
				SSID:      f.SSID,
				Band:      strconv.Itoa(f.Band),
				Channel:   f.Channel,
				Mode:      mode,
				Clients:   nil, // fill below if any
				Timestamp: rowTS,
			}

			var clients []models.FaceClient
			for _, a := range f.Associations {
				st := normMAC(a.Station)
				if st == "" {
					continue
				}

				switch mode {
				case "ap":
					if knownStation[st] == true {
						continue
					}
					// Exclude stations that are any known BSSID; only end-devices remain.
					if _, isBSSID := knownBSSID[st]; isBSSID {
						continue
					}

					fingerprint := ""

					if a.Fingerprint != nil {
						// pick one value in priority order (change order if you want)
						if v, ok := a.Fingerprint["device_name"].(string); ok && v != "" {
							fingerprint = v
						} else if v, ok := a.Fingerprint["vendor"].(string); ok && v != "" {
							fingerprint = v
						} else if v, ok := a.Fingerprint["os"].(string); ok && v != "" {
							fingerprint = v
						} else {
							fingerprint = "unknown"
						}
					}

					// put logic here if a have finger then add it to face clients
					clients = append(clients, models.FaceClient{
						Station:       st,
						RSSI:          a.RSSI,
						Connected:     a.Connected,
						Inactive:      a.Inactive,
						RxRateBitrate: a.RxRate.Bitrate,
						TxRateBitrate: a.TxRate.Bitrate,
						RxRateChwidth: a.RxRate.Chwidth,
						Fingerprint:   fingerprint,
					})
					knownStation[st] = true

				case "mesh":
					// Include only if peer is a known BSSID -> build directed mesh edge serial->peerOwner
					if _, isBSSID := knownBSSID[st]; !isBSSID {
						continue
					}
					clients = append(clients, models.FaceClient{
						Station:       st,
						RSSI:          a.RSSI,
						Connected:     a.Connected,
						Inactive:      a.Inactive,
						RxRateBitrate: a.RxRate.Bitrate,
						TxRateBitrate: a.TxRate.Bitrate,
						RxRateChwidth: a.RxRate.Chwidth,
					})

					if toSerial, ok := bssidOwner[st]; ok && toSerial != "" {
						key := serial + "|" + toSerial + "|" + f.SSID + "|" + strconv.Itoa(f.Band) + "|" + strconv.Itoa(f.Channel)
						if _, seen := meshEdgeSet[key]; !seen {
							meshEdgeSet[key] = models.MeshEdge{
								From:    serial,
								To:      toSerial,
								SSID:    f.SSID,
								Band:    strconv.Itoa(f.Band),
								Channel: f.Channel,
							}
						}
					}
				default:
				}
			}

			// attach slice pointer to make JSON "clients":[...], else keep nil -> JSON null
			if len(clients) > 0 {
				cp := clients
				face.Clients = &cp
			}

			if mode == "ap" {
				dev.APs = append(dev.APs, face)
			} else {
				// keep prior behavior: anything non-"ap" goes under Mesh
				dev.Mesh = append(dev.Mesh, face)
			}
		}

		// deterministic output ordering
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

		devMap[serial] = dev
	}

	// 4) Build final ordered lists: devices by serial, mesh edges
	devs := make([]models.Device, 0, len(devMap))
	for _, d := range devMap {
		devs = append(devs, *d)
	}

	for _, m := range deviceInfo {
		if _, exists := devMap[m.SerialNumber]; !exists {
			devs = append(devs, models.Device{
				Uptime:    0,
				Serial:    m.SerialNumber,
				Connected: m.Connected,
				APs:       []models.Face{},
				Mesh:      []models.Face{},
			})
		}
	}

	sort.Slice(devs, func(i, j int) bool { return devs[i].Serial < devs[j].Serial })

	meshEdges := make([]models.MeshEdge, 0, len(meshEdgeSet))
	for _, e := range meshEdgeSet {
		meshEdges = append(meshEdges, e)
	}
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

	// 5) Final response
	out := models.Topology{
		BoardID:   boardID,
		Timestamp: time.Unix(nowUnix, 0).UTC().Format(time.RFC3339),
		Nodes:     devs,
		Edges:     models.TopoEdges{Wired: []any{}, Mesh: meshEdges},
		External:  []any{},
	}
	s.logger.Info("topology built successfully")
	return out, nil
}

func normMAC(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

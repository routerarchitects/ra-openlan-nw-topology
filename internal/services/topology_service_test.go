package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/router-architects/ra-openlan-nw-topology/internal/models"
	"github.com/routerarchitects/ow-common-mods/servicerpc/analytics"
)

const defaultTimepointIntervalSeconds uint64 = 4 * 60

type mockAnalyticsClient struct {
	timepoints []analytics.TimepointsData
	timeErr    error

	deviceInfo []analytics.DeviceInfo
	deviceErr  error

	macs    []string
	macsErr error

	lastBoardID string
	lastReq     analytics.TimepointRequest
	lastLimit   int
	lastOffset  int
}

func (m *mockAnalyticsClient) GetTimepoints(ctx context.Context, req analytics.TimepointRequest) ([]analytics.TimepointsData, error) {
	m.lastBoardID = req.BoardID
	m.lastReq = req
	if m.timeErr != nil {
		return nil, m.timeErr
	}
	return m.timepoints, nil
}

func (m *mockAnalyticsClient) GetDeviceInfo(ctx context.Context, boardID string) ([]analytics.DeviceInfo, error) {
	m.lastBoardID = boardID
	if m.deviceErr != nil {
		return nil, m.deviceErr
	}
	return m.deviceInfo, nil
}

func (m *mockAnalyticsClient) GetWifiClientHistoryMACs(ctx context.Context, boardID string, limit, offset int) ([]string, error) {
	m.lastBoardID = boardID
	m.lastLimit = limit
	m.lastOffset = offset
	if m.macsErr != nil {
		return nil, m.macsErr
	}
	return m.macs, nil
}

func TestBuildTopology_PropagatesTimepointsError(t *testing.T) {
	wantErr := errors.New("analytics unavailable")
	svc := NewTopologyService(&mockAnalyticsClient{
		timeErr: wantErr,
	})

	_, err := svc.BuildTopology(context.Background(), "board-1", defaultTimepointIntervalSeconds)
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected timepoints error, got %v", err)
	}
}

func TestBuildTopology_UsesRequestedTimeIntervalSeconds(t *testing.T) {
	const intervalSeconds uint64 = 90
	client := &mockAnalyticsClient{}
	svc := NewTopologyService(client)

	minFromDate := uint64(time.Now().Add(-time.Duration(intervalSeconds) * time.Second).Unix())
	_, err := svc.BuildTopology(context.Background(), "board-1", intervalSeconds)
	maxFromDate := uint64(time.Now().Add(-time.Duration(intervalSeconds) * time.Second).Unix())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if client.lastReq.FromDate < minFromDate || client.lastReq.FromDate > maxFromDate {
		t.Fatalf("expected fromDate between %d and %d, got %d", minFromDate, maxFromDate, client.lastReq.FromDate)
	}
}

func TestBuildTopology_EmptyRowsReturnsDeviceInfoNodesAndHistoricalMACs(t *testing.T) {
	client := &mockAnalyticsClient{
		timepoints: []analytics.TimepointsData{},
		deviceInfo: []analytics.DeviceInfo{
			{SerialNumber: "SERIAL-2", Connected: false},
			{SerialNumber: "SERIAL-1", Connected: true},
		},
		macs: []string{
			"aa:bb:cc:dd:ee:ff",
			"AABBCCDDEEFF",
			"11:22:33:44:55:66",
			"",
		},
	}
	svc := NewTopologyService(client)

	minFromDate := uint64(time.Now().Add(-time.Duration(defaultTimepointIntervalSeconds) * time.Second).Unix())
	topology, err := svc.BuildTopology(context.Background(), "board-1", defaultTimepointIntervalSeconds)
	maxFromDate := uint64(time.Now().Add(-time.Duration(defaultTimepointIntervalSeconds) * time.Second).Unix())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if topology.BoardID != "board-1" {
		t.Fatalf("board id mismatch: %q", topology.BoardID)
	}
	if topology.Timestamp == "" {
		t.Fatalf("expected timestamp to be set")
	}
	if _, err := time.Parse(time.RFC3339, topology.Timestamp); err != nil {
		t.Fatalf("expected RFC3339 timestamp, got %q: %v", topology.Timestamp, err)
	}
	if client.lastReq.BoardID != "board-1" {
		t.Fatalf("expected board id to be forwarded to timepoints request, got %q", client.lastReq.BoardID)
	}
	if client.lastReq.FromDate < minFromDate || client.lastReq.FromDate > maxFromDate {
		t.Fatalf("expected fromDate between %d and %d, got %d", minFromDate, maxFromDate, client.lastReq.FromDate)
	}
	if client.lastLimit != 500 || client.lastOffset != 0 {
		t.Fatalf("expected wifi history pagination 500/0, got %d/%d", client.lastLimit, client.lastOffset)
	}

	if len(topology.Nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(topology.Nodes))
	}
	if topology.Nodes[0].Serial != "SERIAL-1" || topology.Nodes[1].Serial != "SERIAL-2" {
		t.Fatalf("unexpected node order/content: %+v", topology.Nodes)
	}
	for _, node := range topology.Nodes {
		if len(node.APs) != 0 || len(node.Mesh) != 0 {
			t.Fatalf("expected empty faces for empty topology rows, got %+v", node)
		}
	}

	wantHistorical := []string{"aa:bb:cc:dd:ee:ff", "11:22:33:44:55:66"}
	if len(topology.HistoricalDevices) != len(wantHistorical) {
		t.Fatalf("expected %d historical devices, got %d", len(wantHistorical), len(topology.HistoricalDevices))
	}
	for i, want := range wantHistorical {
		if topology.HistoricalDevices[i] != want {
			t.Fatalf("historical device %d mismatch: want %q got %q", i, want, topology.HistoricalDevices[i])
		}
	}
	if len(topology.Edges.Mesh) != 0 {
		t.Fatalf("expected no mesh edges, got %+v", topology.Edges.Mesh)
	}
}

func TestBuildTopology_BuildsDevicesEdgesAndHistoricalClients(t *testing.T) {
	client := &mockAnalyticsClient{
		timepoints: []analytics.TimepointsData{
			{
				BoardID:   "board-1",
				Timestamp: 1710000000,
				DeviceInfo: analytics.DeviceInfo{
					SerialNumber: "SERIAL-1",
					Uptime:       101,
				},
				SSIDData: []analytics.SSIDData{
					{
						BSSID:   "AA:AA:AA:AA:AA:01",
						SSID:    "Home-2G",
						Band:    2,
						Channel: 1,
						Mode:    "ap",
						Associations: []analytics.SSIDAssociation{
							{
								Station:   "CC:CC:CC:CC:CC:01",
								RSSI:      -45,
								Connected: 1,
								Inactive:  0,
								RxRate:    analytics.AssociationRate{Bitrate: 100, Chwidth: 20},
								TxRate:    analytics.AssociationRate{Bitrate: 120, Chwidth: 20},
								Fingerprint: map[string]any{
									"device_name": "Pixel",
								},
							},
							{
								Station:   "BB:BB:BB:BB:BB:01",
								RSSI:      -55,
								Connected: 1,
								Inactive:  0,
								RxRate:    analytics.AssociationRate{Bitrate: 200, Chwidth: 40},
								TxRate:    analytics.AssociationRate{Bitrate: 210, Chwidth: 40},
							},
							{
								Station:   "CC:CC:CC:CC:CC:01",
								RSSI:      -47,
								Connected: 1,
								Inactive:  1,
								RxRate:    analytics.AssociationRate{Bitrate: 90, Chwidth: 20},
								TxRate:    analytics.AssociationRate{Bitrate: 95, Chwidth: 20},
							},
						},
					},
					{
						BSSID:   "AA:AA:AA:AA:AA:02",
						SSID:    "Mesh-5G",
						Band:    5,
						Channel: 44,
						Mode:    "mesh",
						Associations: []analytics.SSIDAssociation{
							{
								Station:   "BB:BB:BB:BB:BB:01",
								RSSI:      -60,
								Connected: 1,
								Inactive:  0,
								RxRate:    analytics.AssociationRate{Bitrate: 400, Chwidth: 80},
								TxRate:    analytics.AssociationRate{Bitrate: 350, Chwidth: 80},
							},
						},
					},
				},
			},
			{
				BoardID:   "board-1",
				Timestamp: 1710000030,
				DeviceInfo: analytics.DeviceInfo{
					SerialNumber: "SERIAL-2",
					Uptime:       202,
				},
				SSIDData: []analytics.SSIDData{
					{
						BSSID:   "BB:BB:BB:BB:BB:01",
						SSID:    "Mesh-5G",
						Band:    5,
						Channel: 44,
						Mode:    "mesh",
					},
				},
			},
		},
		deviceInfo: []analytics.DeviceInfo{
			{SerialNumber: "SERIAL-1", Connected: true},
			{SerialNumber: "SERIAL-2", Connected: true},
			{SerialNumber: "SERIAL-3", Connected: false},
		},
		macs: []string{
			"CC:CC:CC:CC:CC:01",
			"DD:DD:DD:DD:DD:01",
			"DDDDDDDDDD01",
			"",
		},
	}
	svc := NewTopologyService(client)

	topology, err := svc.BuildTopology(context.Background(), "board-1", defaultTimepointIntervalSeconds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(topology.Nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(topology.Nodes))
	}

	serial1 := findNode(t, topology.Nodes, "SERIAL-1")
	if !serial1.Connected || serial1.Uptime != 101 {
		t.Fatalf("unexpected SERIAL-1 node: %+v", serial1)
	}
	if len(serial1.APs) != 1 {
		t.Fatalf("expected 1 AP face on SERIAL-1, got %d", len(serial1.APs))
	}
	if len(serial1.Mesh) != 1 {
		t.Fatalf("expected 1 mesh face on SERIAL-1, got %d", len(serial1.Mesh))
	}

	apFace := serial1.APs[0]
	if apFace.BSSID != "aa:aa:aa:aa:aa:01" || apFace.Mode != "ap" {
		t.Fatalf("unexpected AP face: %+v", apFace)
	}
	if apFace.Clients == nil || len(apFace.Clients) != 1 {
		t.Fatalf("expected 1 deduplicated AP client, got %+v", apFace.Clients)
	}
	apClient := (apFace.Clients)[0]
	if apClient.Station != "cc:cc:cc:cc:cc:01" {
		t.Fatalf("unexpected AP client station: %+v", apClient)
	}
	if apClient.Fingerprint != "Pixel" {
		t.Fatalf("expected fingerprint Pixel, got %q", apClient.Fingerprint)
	}

	meshFace := serial1.Mesh[0]
	if meshFace.Clients == nil || len(meshFace.Clients) != 1 {
		t.Fatalf("expected 1 mesh client, got %+v", meshFace.Clients)
	}
	if (meshFace.Clients)[0].Station != "bb:bb:bb:bb:bb:01" {
		t.Fatalf("unexpected mesh client: %+v", (meshFace.Clients)[0])
	}

	serial3 := findNode(t, topology.Nodes, "SERIAL-3")
	if serial3.Connected {
		t.Fatalf("expected SERIAL-3 to remain disconnected")
	}
	if len(serial3.APs) != 0 || len(serial3.Mesh) != 0 {
		t.Fatalf("expected SERIAL-3 to have no faces, got %+v", serial3)
	}

	if len(topology.Edges.Mesh) != 1 {
		t.Fatalf("expected 1 mesh edge, got %+v", topology.Edges.Mesh)
	}
	edge := topology.Edges.Mesh[0]
	if edge.From != "SERIAL-1" || edge.To != "SERIAL-2" || edge.SSID != "Mesh-5G" || edge.Band != "5" || edge.Channel != 44 {
		t.Fatalf("unexpected mesh edge: %+v", edge)
	}

	wantHistorical := []string{"dd:dd:dd:dd:dd:01"}
	if len(topology.HistoricalDevices) != len(wantHistorical) {
		t.Fatalf("expected %d historical devices, got %d", len(wantHistorical), len(topology.HistoricalDevices))
	}
	for i, want := range wantHistorical {
		if topology.HistoricalDevices[i] != want {
			t.Fatalf("historical device %d mismatch: want %q got %q", i, want, topology.HistoricalDevices[i])
		}
	}
}

func TestBuildTopology_IgnoresWifiHistoryError(t *testing.T) {
	client := &mockAnalyticsClient{
		timepoints: []analytics.TimepointsData{
			{
				Timestamp: 1710000000,
				DeviceInfo: analytics.DeviceInfo{
					SerialNumber: "SERIAL-1",
					Uptime:       99,
				},
			},
		},
		deviceInfo: []analytics.DeviceInfo{
			{SerialNumber: "SERIAL-1", Connected: true},
		},
		macsErr: errors.New("history timeout"),
	}
	svc := NewTopologyService(client)

	topology, err := svc.BuildTopology(context.Background(), "board-1", defaultTimepointIntervalSeconds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(topology.Nodes) != 1 {
		t.Fatalf("expected topology to still be built, got %+v", topology.Nodes)
	}
	if len(topology.HistoricalDevices) != 0 {
		t.Fatalf("expected no historical devices on history fetch failure, got %+v", topology.HistoricalDevices)
	}
}

func findNode(t *testing.T, nodes []models.Device, serial string) models.Device {
	t.Helper()
	for _, node := range nodes {
		if node.Serial == serial {
			return node
		}
	}
	t.Fatalf("node %q not found", serial)
	return models.Device{}
}

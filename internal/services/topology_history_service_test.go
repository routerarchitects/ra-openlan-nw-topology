package services

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/router-architects/ra-openlan-nw-topology/internal/config"
	"github.com/routerarchitects/ow-common-mods/servicerpc/analytics"
	"github.com/routerarchitects/ra-common-mods/apperror"
)

type historyMockAnalyticsClient struct {
	getTimepointsFn func(ctx context.Context, req analytics.TimepointRequest) ([]analytics.TimepointsData, error)
	getDeviceInfoFn func(ctx context.Context, boardID string) ([]analytics.DeviceInfo, error)

	timepointCalls []analytics.TimepointRequest
	deviceCalls    int
	wifiCalls      int
}

func (m *historyMockAnalyticsClient) GetTimepoints(ctx context.Context, req analytics.TimepointRequest) ([]analytics.TimepointsData, error) {
	m.timepointCalls = append(m.timepointCalls, req)
	if m.getTimepointsFn != nil {
		return m.getTimepointsFn(ctx, req)
	}
	return nil, nil
}

func (m *historyMockAnalyticsClient) GetDeviceInfo(ctx context.Context, boardID string) ([]analytics.DeviceInfo, error) {
	m.deviceCalls++
	if m.getDeviceInfoFn != nil {
		return m.getDeviceInfoFn(ctx, boardID)
	}
	return nil, nil
}

func (m *historyMockAnalyticsClient) GetWifiClientHistoryMACs(ctx context.Context, boardID string, limit, offset int) ([]string, error) {
	m.wifiCalls++
	return []string{"ff:ff:ff:ff:ff:ff"}, nil
}

func newFixedHistoryService(client AnalyticsClientInterface, nowUnix int64, cfg config.HistoryConfig) *topologyService {
	svc := NewTopologyServiceWithConfig(client, cfg)
	svc.nowFunc = func() time.Time {
		return time.Unix(nowUnix, 0).UTC()
	}
	return svc
}

func TestBuildHistoricalTopology_ValidationAndLimits(t *testing.T) {
	const fixedNow = int64(1795653010)

	tests := []struct {
		name           string
		nowUnix        int64
		cfg            config.HistoryConfig
		boardID        string
		duration       uint64
		windowInterval uint64
		jitter         uint64
		devices        []analytics.DeviceInfo
		wantCode       apperror.Code
	}{
		{
			name:           "missing boardId",
			nowUnix:        fixedNow,
			boardID:        "   ",
			duration:       1800,
			windowInterval: 60,
			jitter:         10,
			wantCode:       apperror.CodeInvalidInput,
		},
		{
			name:           "zero duration",
			nowUnix:        fixedNow,
			boardID:        "board-1",
			duration:       0,
			windowInterval: 60,
			jitter:         10,
			wantCode:       apperror.CodeInvalidInput,
		},
		{
			name:           "zero windowInterval",
			nowUnix:        fixedNow,
			boardID:        "board-1",
			duration:       1800,
			windowInterval: 0,
			jitter:         10,
			wantCode:       apperror.CodeInvalidInput,
		},
		{
			name:           "windowInterval greater than duration",
			nowUnix:        fixedNow,
			boardID:        "board-1",
			duration:       60,
			windowInterval: 120,
			jitter:         10,
			wantCode:       apperror.CodeInvalidInput,
		},
		{
			name:           "excessive duration",
			nowUnix:        fixedNow,
			cfg:            config.HistoryConfig{MaxDurationSeconds: 3600},
			boardID:        "board-1",
			duration:       3601,
			windowInterval: 60,
			jitter:         10,
			wantCode:       apperror.CodeInvalidInput,
		},
		{
			name:           "excessive jitter",
			nowUnix:        fixedNow,
			cfg:            config.HistoryConfig{MaxJitterSeconds: 30},
			boardID:        "board-1",
			duration:       1800,
			windowInterval: 60,
			jitter:         31,
			wantCode:       apperror.CodeInvalidInput,
		},
		{
			name:           "excessive snapshot count",
			nowUnix:        fixedNow,
			cfg:            config.HistoryConfig{MaxSnapshotCount: 10},
			boardID:        "board-1",
			duration:       1800,
			windowInterval: 60,
			jitter:         10,
			wantCode:       apperror.CodeInvalidInput,
		},
		{
			name:           "timestamp underflow on jitter",
			nowUnix:        5,
			boardID:        "board-1",
			duration:       1800,
			windowInterval: 60,
			jitter:         10,
			wantCode:       apperror.CodeInvalidInput,
		},
		{
			name:           "timestamp underflow on duration",
			nowUnix:        100,
			boardID:        "board-1",
			duration:       1800,
			windowInterval: 60,
			jitter:         10,
			wantCode:       apperror.CodeInvalidInput,
		},
		{
			name:           "excessive estimated output units",
			nowUnix:        fixedNow,
			cfg:            config.HistoryConfig{MaxEstimatedOutputUnits: 5},
			boardID:        "board-1",
			duration:       180,
			windowInterval: 60,
			jitter:         10,
			devices: []analytics.DeviceInfo{
				{SerialNumber: "S1"},
				{SerialNumber: "S2"},
			},
			wantCode: apperror.CodeInvalidInput,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := &historyMockAnalyticsClient{
				getDeviceInfoFn: func(ctx context.Context, boardID string) ([]analytics.DeviceInfo, error) {
					return tc.devices, nil
				},
			}
			svc := newFixedHistoryService(client, tc.nowUnix, tc.cfg)
			_, err := svc.BuildHistoricalTopology(context.Background(), tc.boardID, tc.duration, tc.windowInterval, tc.jitter)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if apperror.CodeOf(err) != tc.wantCode {
				t.Fatalf("expected error code %v, got %v (%v)", tc.wantCode, apperror.CodeOf(err), err)
			}
		})
	}
}

func TestBuildHistoricalTopology_RangeCalculationAndSingleTimeCapture(t *testing.T) {
	const fixedNow = int64(1795653010) // historyEnd = 1795653000, historyStart = 1795651200 for 1800s

	tests := []struct {
		name             string
		duration         uint64
		windowInterval   uint64
		jitter           uint64
		wantSnapshots    int
		wantFromDate     int64
		wantEndDate      int64
		wantFetchEndDate uint64
	}{
		{
			name:             "1800s / 60s = 30 snapshots",
			duration:         1800,
			windowInterval:   60,
			jitter:           10,
			wantSnapshots:    30,
			wantFromDate:     1795651200,
			wantEndDate:      1795653000,
			wantFetchEndDate: 1795653010,
		},
		{
			name:             "1800s / 120s = 15 snapshots",
			duration:         1800,
			windowInterval:   120,
			jitter:           10,
			wantSnapshots:    15,
			wantFromDate:     1795651200,
			wantEndDate:      1795653000,
			wantFetchEndDate: 1795653010,
		},
		{
			name:             "non-divisible 1850s / 60s omits trailing 50s",
			duration:         1850,
			windowInterval:   60,
			jitter:           10,
			wantSnapshots:    30,
			wantFromDate:     1795651150,
			wantEndDate:      1795652950,
			wantFetchEndDate: 1795652960,
		},
		{
			name:             "explicit zero jitter",
			duration:         1800,
			windowInterval:   60,
			jitter:           0,
			wantSnapshots:    30,
			wantFromDate:     1795651210,
			wantEndDate:      1795653010,
			wantFetchEndDate: 1795653010,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			nowCalls := 0
			client := &historyMockAnalyticsClient{
				getDeviceInfoFn: func(ctx context.Context, boardID string) ([]analytics.DeviceInfo, error) {
					return []analytics.DeviceInfo{{SerialNumber: "AP-1"}}, nil
				},
			}
			svc := NewTopologyServiceWithConfig(client, config.DefaultHistoryConfig())
			svc.nowFunc = func() time.Time {
				nowCalls++
				return time.Unix(fixedNow, 0).UTC()
			}

			resp, err := svc.BuildHistoricalTopology(context.Background(), "board-1", tc.duration, tc.windowInterval, tc.jitter)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if nowCalls != 1 {
				t.Fatalf("expected nowFunc to be called once, got %d", nowCalls)
			}
			if resp.SnapshotCount != tc.wantSnapshots || len(resp.Snapshots) != tc.wantSnapshots {
				t.Fatalf("expected %d snapshots, got count=%d len=%d", tc.wantSnapshots, resp.SnapshotCount, len(resp.Snapshots))
			}
			if resp.FromDate != tc.wantFromDate || resp.EndDate != tc.wantEndDate {
				t.Fatalf("range mismatch: got [%d, %d], want [%d, %d]", resp.FromDate, resp.EndDate, tc.wantFromDate, tc.wantEndDate)
			}
			lastSnap := resp.Snapshots[len(resp.Snapshots)-1]
			if lastSnap.EffectiveWindowEnd > fixedNow {
				t.Fatalf("final effectiveWindowEnd %d exceeds now %d", lastSnap.EffectiveWindowEnd, fixedNow)
			}
			if len(client.timepointCalls) != 1 {
				t.Fatalf("expected 1 timepoint call, got %d", len(client.timepointCalls))
			}
			req := client.timepointCalls[0]
			if req.LatestPerDevice == nil || *req.LatestPerDevice != false {
				t.Fatalf("expected LatestPerDevice=false, got %v", req.LatestPerDevice)
			}
			if req.PointsOnly == nil || *req.PointsOnly != true {
				t.Fatalf("expected PointsOnly=true, got %v", req.PointsOnly)
			}
			if req.StatsOnly == nil || *req.StatsOnly != false {
				t.Fatalf("expected StatsOnly=false, got %v", req.StatsOnly)
			}
			if req.FromDate != uint64(tc.wantFromDate) || req.EndDate != tc.wantFetchEndDate {
				t.Fatalf("fetch range mismatch: got [%d, %d], want [%d, %d]", req.FromDate, req.EndDate, tc.wantFromDate, tc.wantFetchEndDate)
			}
			if client.wifiCalls != 0 {
				t.Fatalf("expected 0 wifi history calls for historical topology, got %d", client.wifiCalls)
			}
		})
	}
}

func TestBuildHistoricalTopology_SlidingWindowBoundariesAndOverlap(t *testing.T) {
	// window 0: [1000, 1060], effective [1000, 1070]
	// window 1: [1060, 1120], effective [1060, 1130]
	// now = 1120 + 10 = 1130, duration = 120, interval = 60, jitter = 10 -> historyStart = 1000
	const nowUnix = int64(1130)

	cases := []struct {
		name     string
		ts       int64
		wantWin0 bool
		wantWin1 bool
	}{
		{name: "999 excluded", ts: 999, wantWin0: false, wantWin1: false},
		{name: "1000 included in win0 only", ts: 1000, wantWin0: true, wantWin1: false},
		{name: "1059 included in win0 only", ts: 1059, wantWin0: true, wantWin1: false},
		{name: "1060 included in both", ts: 1060, wantWin0: true, wantWin1: true},
		{name: "1065 overlap included in both", ts: 1065, wantWin0: true, wantWin1: true},
		{name: "1070 included in both", ts: 1070, wantWin0: true, wantWin1: true},
		{name: "1071 included in win1 only", ts: 1071, wantWin0: false, wantWin1: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := &historyMockAnalyticsClient{
				getDeviceInfoFn: func(ctx context.Context, boardID string) ([]analytics.DeviceInfo, error) {
					return []analytics.DeviceInfo{{SerialNumber: "AP-1", Connected: false}}, nil
				},
				getTimepointsFn: func(ctx context.Context, req analytics.TimepointRequest) ([]analytics.TimepointsData, error) {
					return []analytics.TimepointsData{
						{
							ID:        "tp-1",
							BoardID:   "board-1",
							Timestamp: tc.ts,
							Serial:    "AP-1",
						},
					}, nil
				},
			}
			svc := newFixedHistoryService(client, nowUnix, config.DefaultHistoryConfig())
			resp, err := svc.BuildHistoricalTopology(context.Background(), "board-1", 120, 60, 10)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resp.Snapshots[0].Nodes[0].Connected != tc.wantWin0 {
				t.Fatalf("win0 connected=%v, want %v", resp.Snapshots[0].Nodes[0].Connected, tc.wantWin0)
			}
			if resp.Snapshots[1].Nodes[0].Connected != tc.wantWin1 {
				t.Fatalf("win1 connected=%v, want %v", resp.Snapshots[1].Nodes[0].Connected, tc.wantWin1)
			}
		})
	}
}

func TestBuildHistoricalTopology_AvailabilityAndTopologyReconstruction(t *testing.T) {
	// 3 windows of 60s with 10s jitter starting at 1000:
	// win0: [1000, 1060] eff 1070
	// win1: [1060, 1120] eff 1130
	// win2: [1120, 1180] eff 1190
	// now = 1190, duration = 180, interval = 60, jitter = 10
	const nowUnix = int64(1190)

	client := &historyMockAnalyticsClient{
		getDeviceInfoFn: func(ctx context.Context, boardID string) ([]analytics.DeviceInfo, error) {
			return []analytics.DeviceInfo{
				{SerialNumber: "AP-1", Connected: false}, // currently disconnected, historically online in win0 & win2
				{SerialNumber: "AP-2", Connected: true},  // currently connected, historically online in win0 only
				{SerialNumber: "AP-3", Connected: true},  // currently connected, no historical records -> always offline
				{SerialNumber: "OTHER", BoardID: "other-board", Connected: true},
			}, nil
		},
		getTimepointsFn: func(ctx context.Context, req analytics.TimepointRequest) ([]analytics.TimepointsData, error) {
			// Intentionally unsorted, with duplicate ID and out-of-scope device.
			return []analytics.TimepointsData{
				{
					ID:        "tp-ap1-win2",
					BoardID:   "board-1",
					Timestamp: 1150,
					Serial:    "AP-1",
					SSIDData: []analytics.SSIDData{
						{
							BSSID: "aa:00:00:00:00:01", SSID: "Corp", Band: 5, Channel: 36, Mode: "ap",
							Associations: []analytics.SSIDAssociation{
								{Station: "cc:00:00:00:00:01", RSSI: -50},
							},
						},
					},
				},
				{
					ID:        "tp-ap1-win0",
					BoardID:   "board-1",
					Timestamp: 1015,
					Serial:    "AP-1",
					SSIDData: []analytics.SSIDData{
						{
							BSSID: "aa:00:00:00:00:01", SSID: "Corp", Band: 5, Channel: 36, Mode: "ap",
							Associations: []analytics.SSIDAssociation{
								{Station: "cc:00:00:00:00:01", RSSI: -60},
							},
						},
						{
							BSSID: "aa:00:00:00:00:02", SSID: "Mesh", Band: 5, Channel: 149, Mode: "mesh",
							Associations: []analytics.SSIDAssociation{
								{Station: "bb:00:00:00:00:02", RSSI: -40},
							},
						},
					},
				},
				// Duplicate of tp-ap1-win0
				{
					ID:        "tp-ap1-win0",
					BoardID:   "board-1",
					Timestamp: 1015,
					Serial:    "AP-1",
					SSIDData: []analytics.SSIDData{
						{
							BSSID: "aa:00:00:00:00:01", SSID: "Corp", Band: 5, Channel: 36, Mode: "ap",
							Associations: []analytics.SSIDAssociation{
								{Station: "cc:00:00:00:00:01", RSSI: -60},
							},
						},
						{
							BSSID: "aa:00:00:00:00:02", SSID: "Mesh", Band: 5, Channel: 149, Mode: "mesh",
							Associations: []analytics.SSIDAssociation{
								{Station: "bb:00:00:00:00:02", RSSI: -40},
							},
						},
					},
				},
				// AP-2 online in win0, roaming client cc:00:00:00:00:01 at newer timestamp 1030
				{
					ID:        "tp-ap2-win0",
					BoardID:   "board-1",
					Timestamp: 1030,
					Serial:    "AP-2",
					SSIDData: []analytics.SSIDData{
						{
							BSSID: "bb:00:00:00:00:01", SSID: "Corp", Band: 5, Channel: 36, Mode: "ap",
							Associations: []analytics.SSIDAssociation{
								{Station: "cc:00:00:00:00:01", RSSI: -42},
							},
						},
						{
							BSSID: "bb:00:00:00:00:02", SSID: "Mesh", Band: 5, Channel: 149, Mode: "mesh",
						},
					},
				},
				// Unauthorized / non-member device should be excluded
				{
					ID:        "tp-rogue",
					BoardID:   "board-1",
					Timestamp: 1020,
					Serial:    "ROGUE-AP",
				},
				// Another board record should be excluded
				{
					ID:        "tp-other-board",
					BoardID:   "board-2",
					Timestamp: 1020,
					Serial:    "AP-3",
				},
			}, nil
		},
	}

	svc := newFixedHistoryService(client, nowUnix, config.DefaultHistoryConfig())
	resp, err := svc.BuildHistoricalTopology(context.Background(), "board-1", 180, 60, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Window 0: AP-1 online, AP-2 online, AP-3 offline. Mesh edge AP-1 -> AP-2 exists.
	// Client cc:00:00:00:00:01 roamed to AP-2 (ts 1030 > 1015), so AP-2 has client, AP-1 does not.
	win0 := resp.Snapshots[0]
	if len(win0.Nodes) != 3 {
		t.Fatalf("expected 3 authorized nodes in win0, got %d", len(win0.Nodes))
	}
	if !win0.Nodes[0].Connected || !win0.Nodes[1].Connected || win0.Nodes[2].Connected {
		t.Fatalf("unexpected win0 statuses: %+v", win0.Nodes)
	}
	if len(win0.Edges.Mesh) != 1 || win0.Edges.Mesh[0].From != "AP-1" || win0.Edges.Mesh[0].To != "AP-2" {
		t.Fatalf("unexpected win0 mesh edges: %+v", win0.Edges.Mesh)
	}
	if len(win0.Nodes[0].APs[0].Clients) != 0 {
		t.Fatalf("expected roamed client to be removed from AP-1 in win0, got %+v", win0.Nodes[0].APs[0].Clients)
	}
	if len(win0.Nodes[1].APs[0].Clients) != 1 || win0.Nodes[1].APs[0].Clients[0].Station != "cc:00:00:00:00:01" {
		t.Fatalf("expected roamed client on AP-2 in win0, got %+v", win0.Nodes[1].APs[0].Clients)
	}

	// Window 1: All offline! No mesh edges, empty aps/mesh.
	win1 := resp.Snapshots[1]
	for _, n := range win1.Nodes {
		if n.Connected || len(n.APs) != 0 || len(n.Mesh) != 0 {
			t.Fatalf("expected offline empty node in win1, got %+v", n)
		}
	}
	if len(win1.Edges.Mesh) != 0 {
		t.Fatalf("expected no mesh edges in win1, got %+v", win1.Edges.Mesh)
	}

	// Window 2: AP-1 online again, AP-2 offline, AP-3 offline.
	win2 := resp.Snapshots[2]
	if !win2.Nodes[0].Connected || win2.Nodes[1].Connected || win2.Nodes[2].Connected {
		t.Fatalf("unexpected win2 statuses: %+v", win2.Nodes)
	}
	if len(win2.Nodes[0].APs) != 1 || len(win2.Nodes[0].APs[0].Clients) != 1 {
		t.Fatalf("expected AP-1 to have client in win2, got %+v", win2.Nodes[0])
	}
}

func TestBuildHistoricalTopology_PartitioningAndErrorPropagation(t *testing.T) {
	t.Run("partitions range when hitting MaxTimepointsPerFetch", func(t *testing.T) {
		const nowUnix = int64(1130)
		cfg := config.HistoryConfig{
			MaxTimepointsPerFetch: 2,
			MaxTotalTimepoints:    100,
		}
		client := &historyMockAnalyticsClient{
			getDeviceInfoFn: func(ctx context.Context, boardID string) ([]analytics.DeviceInfo, error) {
				return []analytics.DeviceInfo{{SerialNumber: "AP-1"}}, nil
			},
			getTimepointsFn: func(ctx context.Context, req analytics.TimepointRequest) ([]analytics.TimepointsData, error) {
				if req.FromDate == 1000 && req.EndDate == 1130 {
					// Return exactly MaxTimepointsPerFetch=2 to trigger split
					return []analytics.TimepointsData{
						{ID: "1", Timestamp: 1010, Serial: "AP-1"},
						{ID: "2", Timestamp: 1090, Serial: "AP-1"},
					}, nil
				}
				if req.FromDate <= 1065 && req.EndDate <= 1065 {
					return []analytics.TimepointsData{{ID: "1", Timestamp: 1010, Serial: "AP-1"}}, nil
				}
				return []analytics.TimepointsData{{ID: "2", Timestamp: 1090, Serial: "AP-1"}}, nil
			},
		}
		svc := newFixedHistoryService(client, nowUnix, cfg)
		resp, err := svc.BuildHistoricalTopology(context.Background(), "board-1", 120, 60, 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(client.timepointCalls) != 3 {
			t.Fatalf("expected 3 partitioned timepoint calls, got %d", len(client.timepointCalls))
		}
		if !resp.Snapshots[0].Nodes[0].Connected || !resp.Snapshots[1].Nodes[0].Connected {
			t.Fatalf("expected AP-1 online in both snapshots")
		}
	})

	t.Run("fails explicitly on single-second range truncation", func(t *testing.T) {
		const nowUnix = int64(1061)
		cfg := config.HistoryConfig{
			MaxTimepointsPerFetch: 1,
			MaxTotalTimepoints:    100,
		}
		client := &historyMockAnalyticsClient{
			getDeviceInfoFn: func(ctx context.Context, boardID string) ([]analytics.DeviceInfo, error) {
				return []analytics.DeviceInfo{{SerialNumber: "AP-1"}}, nil
			},
			getTimepointsFn: func(ctx context.Context, req analytics.TimepointRequest) ([]analytics.TimepointsData, error) {
				return []analytics.TimepointsData{{ID: "1", Timestamp: int64(req.FromDate), Serial: "AP-1"}}, nil
			},
		}
		svc := newFixedHistoryService(client, nowUnix, cfg)
		_, err := svc.BuildHistoricalTopology(context.Background(), "board-1", 1, 1, 0)
		if err == nil {
			t.Fatalf("expected error on single-second truncation, got nil")
		}
		if apperror.CodeOf(err) != apperror.CodeInternal {
			t.Fatalf("expected CodeInternal, got %v", apperror.CodeOf(err))
		}
	})

	t.Run("propagates membership and timepoint errors", func(t *testing.T) {
		wantErr := apperror.New(apperror.CodeNotFound, "board not found")
		client := &historyMockAnalyticsClient{
			getDeviceInfoFn: func(ctx context.Context, boardID string) ([]analytics.DeviceInfo, error) {
				return nil, wantErr
			},
		}
		svc := newFixedHistoryService(client, 1795653010, config.DefaultHistoryConfig())
		_, err := svc.BuildHistoricalTopology(context.Background(), "board-1", 1800, 60, 10)
		if !errors.Is(err, wantErr) {
			t.Fatalf("expected %v, got %v", wantErr, err)
		}

		tpErr := apperror.New(apperror.CodeInternal, "analytics down")
		client2 := &historyMockAnalyticsClient{
			getDeviceInfoFn: func(ctx context.Context, boardID string) ([]analytics.DeviceInfo, error) {
				return []analytics.DeviceInfo{{SerialNumber: "AP-1"}}, nil
			},
			getTimepointsFn: func(ctx context.Context, req analytics.TimepointRequest) ([]analytics.TimepointsData, error) {
				return nil, tpErr
			},
		}
		svc2 := newFixedHistoryService(client2, 1795653010, config.DefaultHistoryConfig())
		_, err = svc2.BuildHistoricalTopology(context.Background(), "board-1", 1800, 60, 10)
		if !errors.Is(err, tpErr) {
			t.Fatalf("expected %v, got %v", tpErr, err)
		}
	})

	t.Run("rejects conflicting serial identifiers and conflicting duplicate IDs", func(t *testing.T) {
		client := &historyMockAnalyticsClient{
			getDeviceInfoFn: func(ctx context.Context, boardID string) ([]analytics.DeviceInfo, error) {
				return []analytics.DeviceInfo{{SerialNumber: "AP-1"}}, nil
			},
			getTimepointsFn: func(ctx context.Context, req analytics.TimepointRequest) ([]analytics.TimepointsData, error) {
				return []analytics.TimepointsData{
					{
						ID:         "tp-1",
						Timestamp:  1795651210,
						Serial:     "AP-1",
						DeviceInfo: analytics.DeviceInfo{SerialNumber: "AP-2"},
					},
				}, nil
			},
		}
		svc := newFixedHistoryService(client, 1795653010, config.DefaultHistoryConfig())
		if _, err := svc.BuildHistoricalTopology(context.Background(), "board-1", 1800, 60, 10); err == nil {
			t.Fatalf("expected error on conflicting serials")
		}

		client2 := &historyMockAnalyticsClient{
			getDeviceInfoFn: func(ctx context.Context, boardID string) ([]analytics.DeviceInfo, error) {
				return []analytics.DeviceInfo{{SerialNumber: "AP-1"}}, nil
			},
			getTimepointsFn: func(ctx context.Context, req analytics.TimepointRequest) ([]analytics.TimepointsData, error) {
				return []analytics.TimepointsData{
					{ID: "tp-1", Timestamp: 1795651210, Serial: "AP-1"},
					{ID: "tp-1", Timestamp: 1795651220, Serial: "AP-1"},
				}, nil
			},
		}
		svc2 := newFixedHistoryService(client2, 1795653010, config.DefaultHistoryConfig())
		if _, err := svc2.BuildHistoricalTopology(context.Background(), "board-1", 1800, 60, 10); apperror.CodeOf(err) != apperror.CodeConflict {
			t.Fatalf("expected CodeConflict on conflicting duplicate IDs, got %v", err)
		}

		// Same timestamp/serial/boardId but different SSIDData/Associations should also be rejected as CodeConflict.
		client3 := &historyMockAnalyticsClient{
			getDeviceInfoFn: func(ctx context.Context, boardID string) ([]analytics.DeviceInfo, error) {
				return []analytics.DeviceInfo{{SerialNumber: "AP-1"}}, nil
			},
			getTimepointsFn: func(ctx context.Context, req analytics.TimepointRequest) ([]analytics.TimepointsData, error) {
				return []analytics.TimepointsData{
					{
						ID:        "tp-1",
						BoardID:   "board-1",
						Timestamp: 1795651210,
						Serial:    "AP-1",
						SSIDData: []analytics.SSIDData{
							{BSSID: "aa:00:00:00:00:01", SSID: "Corp", Band: 5, Channel: 36, Mode: "ap"},
						},
					},
					{
						ID:        "tp-1",
						BoardID:   "board-1",
						Timestamp: 1795651210,
						Serial:    "AP-1",
						SSIDData: []analytics.SSIDData{
							{
								BSSID: "aa:00:00:00:00:01", SSID: "Corp", Band: 5, Channel: 36, Mode: "ap",
								Associations: []analytics.SSIDAssociation{{Station: "cc:00:00:00:00:01", RSSI: -55}},
							},
						},
					},
				}, nil
			},
		}
		svc3 := newFixedHistoryService(client3, 1795653010, config.DefaultHistoryConfig())
		if _, err := svc3.BuildHistoricalTopology(context.Background(), "board-1", 1800, 60, 10); apperror.CodeOf(err) != apperror.CodeConflict {
			t.Fatalf("expected CodeConflict on conflicting duplicate ID payload, got %v", err)
		}
	})

	t.Run("enforces output unit limit including interfaces and clients", func(t *testing.T) {
		const nowUnix = int64(1130)
		// 2 snapshots x 1 device = 2 nodes (passes pre-fetch check with limit=5),
		// but 1 AP + 3 clients per snapshot = 12 total output units > limit 5.
		cfg := config.HistoryConfig{
			MaxEstimatedOutputUnits: 5,
		}
		client := &historyMockAnalyticsClient{
			getDeviceInfoFn: func(ctx context.Context, boardID string) ([]analytics.DeviceInfo, error) {
				return []analytics.DeviceInfo{{SerialNumber: "AP-1"}}, nil
			},
			getTimepointsFn: func(ctx context.Context, req analytics.TimepointRequest) ([]analytics.TimepointsData, error) {
				return []analytics.TimepointsData{
					{
						ID:        "tp-1",
						BoardID:   "board-1",
						Timestamp: 1065, // falls in both snapshots
						Serial:    "AP-1",
						SSIDData: []analytics.SSIDData{
							{
								BSSID: "aa:00:00:00:00:01", SSID: "Corp", Band: 5, Channel: 36, Mode: "ap",
								Associations: []analytics.SSIDAssociation{
									{Station: "cc:00:00:00:00:01", RSSI: -50},
									{Station: "cc:00:00:00:00:02", RSSI: -55},
									{Station: "cc:00:00:00:00:03", RSSI: -60},
								},
							},
						},
					},
				}, nil
			},
		}
		svc := newFixedHistoryService(client, nowUnix, cfg)
		_, err := svc.BuildHistoricalTopology(context.Background(), "board-1", 120, 60, 10)
		if apperror.CodeOf(err) != apperror.CodeInvalidInput {
			t.Fatalf("expected CodeInvalidInput when output units exceed limit, got %v", err)
		}
	})
}

func benchmarkHistoricalTopology(b *testing.B, deviceCount int) {
	const (
		nowUnix        = int64(1795653010)
		duration       = uint64(1800)
		windowInterval = uint64(60)
		jitter         = uint64(10)
	)
	historyStart := nowUnix - int64(jitter) - int64(duration)
	snapshots := int(duration / windowInterval)

	devices := make([]analytics.DeviceInfo, deviceCount)
	rows := make([]analytics.TimepointsData, 0, deviceCount*snapshots)

	for d := 0; d < deviceCount; d++ {
		serial := fmt.Sprintf("AP-%04d", d)
		devices[d] = analytics.DeviceInfo{SerialNumber: serial, Connected: true}

		meshPeerBSSID := fmt.Sprintf("ee:00:00:%02x:%02x:01", ((d+1)/256)%256, (d+1)%256)
		ownMeshBSSID := fmt.Sprintf("ee:00:00:%02x:%02x:01", (d/256)%256, d%256)
		apBSSID := fmt.Sprintf("aa:00:00:%02x:%02x:01", (d/256)%256, d%256)

		for s := 0; s < snapshots; s++ {
			ts := historyStart + int64(s)*int64(windowInterval) + 15
			rows = append(rows, analytics.TimepointsData{
				ID:        fmt.Sprintf("tp-%d-%d", d, s),
				BoardID:   "bench-board",
				Timestamp: ts,
				Serial:    serial,
				DeviceInfo: analytics.DeviceInfo{
					SerialNumber: serial,
					Uptime:       int64(1000 + s*60),
				},
				SSIDData: []analytics.SSIDData{
					{
						BSSID:   apBSSID,
						SSID:    "Bench-WiFi",
						Band:    5,
						Channel: 36,
						Mode:    "ap",
						Associations: []analytics.SSIDAssociation{
							{Station: fmt.Sprintf("cc:00:%02x:%02x:00:01", (d/256)%256, d%256), RSSI: -55},
							{Station: fmt.Sprintf("cc:00:%02x:%02x:00:02", (d/256)%256, d%256), RSSI: -62},
						},
					},
					{
						BSSID:   ownMeshBSSID,
						SSID:    "Bench-Mesh",
						Band:    5,
						Channel: 149,
						Mode:    "mesh",
						Associations: []analytics.SSIDAssociation{
							{Station: meshPeerBSSID, RSSI: -48},
						},
					},
				},
			})
		}
	}

	client := &historyMockAnalyticsClient{
		getDeviceInfoFn: func(ctx context.Context, boardID string) ([]analytics.DeviceInfo, error) {
			return devices, nil
		},
		getTimepointsFn: func(ctx context.Context, req analytics.TimepointRequest) ([]analytics.TimepointsData, error) {
			return rows, nil
		},
	}

	cfg := config.DefaultHistoryConfig()
	cfg.MaxTimepointsPerFetch = len(rows) + 100
	svc := newFixedHistoryService(client, nowUnix, cfg)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := svc.BuildHistoricalTopology(context.Background(), "bench-board", duration, windowInterval, jitter)
		if err != nil {
			b.Fatalf("benchmark failed: %v", err)
		}
		if resp.SnapshotCount != snapshots {
			b.Fatalf("expected %d snapshots, got %d", snapshots, resp.SnapshotCount)
		}
	}
}

func BenchmarkBuildHistoricalTopology_10Devices(b *testing.B)   { benchmarkHistoricalTopology(b, 10) }
func BenchmarkBuildHistoricalTopology_100Devices(b *testing.B)  { benchmarkHistoricalTopology(b, 100) }
func BenchmarkBuildHistoricalTopology_500Devices(b *testing.B)  { benchmarkHistoricalTopology(b, 500) }
func BenchmarkBuildHistoricalTopology_1000Devices(b *testing.B) { benchmarkHistoricalTopology(b, 1000) }

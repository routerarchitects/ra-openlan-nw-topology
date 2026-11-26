package handlers_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/router-architects/ra-openlan-nw-topology/internal/http/handlers"
	"github.com/router-architects/ra-openlan-nw-topology/internal/http/router"
	"github.com/router-architects/ra-openlan-nw-topology/internal/models"
	subsysteroutes "github.com/routerarchitects/ow-common-mods/fiber/system-routes"
	"github.com/routerarchitects/ra-common-mods/apperror"
)

type mockTopologyService struct {
	currentResp *models.Topology
	currentErr  error

	historyResp *models.HistoricalTopologyResponse
	historyErr  error

	calledHistory bool
	lastBoardID   string
	lastDuration  uint64
	lastInterval  uint64
	lastJitter    uint64
}

func (m *mockTopologyService) BuildTopology(ctx context.Context, boardID string, timeIntervalSeconds uint64) (*models.Topology, error) {
	if m.currentErr != nil {
		return nil, m.currentErr
	}
	return m.currentResp, nil
}

func (m *mockTopologyService) BuildHistoricalTopology(
	ctx context.Context,
	boardID string,
	duration uint64,
	windowInterval uint64,
	jitter uint64,
) (*models.HistoricalTopologyResponse, error) {
	m.calledHistory = true
	m.lastBoardID = boardID
	m.lastDuration = duration
	m.lastInterval = windowInterval
	m.lastJitter = jitter
	if m.historyErr != nil {
		return nil, m.historyErr
	}
	return m.historyResp, nil
}

func TestGetHistoricalTopology_ValidationAndDefaults(t *testing.T) {
	tests := []struct {
		name         string
		query        string
		wantStatus   int
		wantCalled   bool
		wantBoardID  string
		wantDuration uint64
		wantInterval uint64
		wantJitter   uint64
	}{
		{
			name:       "missing boardId",
			query:      "?windowInterval=60",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "blank boardId",
			query:      "?boardId=%20%20&windowInterval=60",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing windowInterval",
			query:      "?boardId=board-1",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "zero windowInterval",
			query:      "?boardId=board-1&windowInterval=0",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "negative windowInterval",
			query:      "?boardId=board-1&windowInterval=-60",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "zero duration",
			query:      "?boardId=board-1&duration=0&windowInterval=60",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "negative duration",
			query:      "?boardId=board-1&duration=-100&windowInterval=60",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "negative jitter",
			query:      "?boardId=board-1&windowInterval=60&jitter=-1",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "non-integer duration",
			query:      "?boardId=board-1&duration=abc&windowInterval=60",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "non-integer windowInterval",
			query:      "?boardId=board-1&windowInterval=abc",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "non-integer jitter",
			query:      "?boardId=board-1&windowInterval=60&jitter=xyz",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "windowInterval greater than duration",
			query:      "?boardId=board-1&duration=60&windowInterval=120",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:         "defaults for duration and jitter",
			query:        "?boardId=%20board-1%20&windowInterval=60",
			wantStatus:   http.StatusOK,
			wantCalled:   true,
			wantBoardID:  "board-1",
			wantDuration: 1800,
			wantInterval: 60,
			wantJitter:   10,
		},
		{
			name:         "explicit zero jitter",
			query:        "?boardId=board-1&duration=1200&windowInterval=60&jitter=0",
			wantStatus:   http.StatusOK,
			wantCalled:   true,
			wantBoardID:  "board-1",
			wantDuration: 1200,
			wantInterval: 60,
			wantJitter:   0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := &mockTopologyService{
				historyResp: &models.HistoricalTopologyResponse{
					BoardID:       "board-1",
					SnapshotCount: 1,
					Snapshots:     []models.HistoricalTopologySnapshot{},
				},
			}
			app := fiber.New()
			h := handlers.NewTopologyHandler(svc)
			app.Get("/api/v1/topology/history", h.GetHistoricalTopology)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/topology/history"+tc.query, nil)
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("app.Test failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tc.wantStatus {
				body, _ := io.ReadAll(resp.Body)
				t.Fatalf("expected status %d, got %d (body: %s)", tc.wantStatus, resp.StatusCode, string(body))
			}
			if svc.calledHistory != tc.wantCalled {
				t.Fatalf("expected calledHistory=%v, got %v", tc.wantCalled, svc.calledHistory)
			}
			if tc.wantCalled {
				if svc.lastBoardID != tc.wantBoardID || svc.lastDuration != tc.wantDuration || svc.lastInterval != tc.wantInterval || svc.lastJitter != tc.wantJitter {
					t.Fatalf("unexpected service args: board=%q dur=%d interval=%d jitter=%d",
						svc.lastBoardID, svc.lastDuration, svc.lastInterval, svc.lastJitter)
				}
			}
		})
	}
}

func TestGetHistoricalTopology_ErrorMappingAndEmptyArraysSerialization(t *testing.T) {
	errCases := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{name: "unauthorized", err: apperror.New(apperror.CodeUnauthorized, "unauthorized"), wantStatus: http.StatusUnauthorized},
		{name: "forbidden", err: apperror.New(apperror.CodeForbidden, "forbidden"), wantStatus: http.StatusForbidden},
		{name: "not found", err: apperror.New(apperror.CodeNotFound, "not found"), wantStatus: http.StatusNotFound},
		{name: "invalid input", err: apperror.New(apperror.CodeInvalidInput, "too large"), wantStatus: http.StatusBadRequest},
		{name: "internal", err: apperror.New(apperror.CodeInternal, "boom"), wantStatus: http.StatusInternalServerError},
	}

	for _, tc := range errCases {
		t.Run(tc.name, func(t *testing.T) {
			svc := &mockTopologyService{historyErr: tc.err}
			app := fiber.New()
			h := handlers.NewTopologyHandler(svc)
			app.Get("/api/v1/topology/history", h.GetHistoricalTopology)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/topology/history?boardId=b1&windowInterval=60", nil)
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("app.Test failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tc.wantStatus {
				t.Fatalf("expected status %d, got %d", tc.wantStatus, resp.StatusCode)
			}
		})
	}

	t.Run("serializes explicit empty arrays for offline nodes", func(t *testing.T) {
		svc := &mockTopologyService{
			historyResp: &models.HistoricalTopologyResponse{
				BoardID:        "b1",
				FromDate:       1000,
				EndDate:        1060,
				Duration:       60,
				WindowInterval: 60,
				Jitter:         10,
				SnapshotCount:  1,
				Snapshots: []models.HistoricalTopologySnapshot{
					{
						Timestamp:          1060,
						WindowStart:        1000,
						WindowEnd:          1060,
						EffectiveWindowEnd: 1070,
						Nodes: []models.HistoricalDevice{
							{
								Serial:    "dev-1",
								Connected: false,
								APs:       []models.Iface{},
								Mesh:      []models.Iface{},
							},
						},
						Edges: models.TopoEdges{
							Wired: []any{},
							Mesh:  []models.MeshEdge{},
						},
					},
				},
			},
		}
		app := fiber.New()
		h := handlers.NewTopologyHandler(svc)
		app.Get("/api/v1/topology/history", h.GetHistoricalTopology)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/topology/history?boardId=b1&duration=60&windowInterval=60", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test failed: %v", err)
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		var decoded map[string]any
		if err := json.Unmarshal(body, &decoded); err != nil {
			t.Fatalf("invalid json: %v", err)
		}
		snaps := decoded["snapshots"].([]any)
		snap0 := snaps[0].(map[string]any)
		nodes := snap0["nodes"].([]any)
		node0 := nodes[0].(map[string]any)
		if _, ok := node0["aps"]; !ok {
			t.Fatalf("expected explicit aps field in JSON, got %s", string(body))
		}
		if _, ok := node0["mesh"]; !ok {
			t.Fatalf("expected explicit mesh field in JSON, got %s", string(body))
		}
	})
}

func TestRouter_RegistersHistoricalEndpointWithAuthOnPublicAndPrivate(t *testing.T) {
	svc := &mockTopologyService{
		currentResp: &models.Topology{BoardID: "b1"},
		historyResp: &models.HistoricalTopologyResponse{BoardID: "b1", Snapshots: []models.HistoricalTopologySnapshot{}},
	}
	h := handlers.NewTopologyHandler(svc)

	rejectAuth := func(c fiber.Ctx) error {
		if c.Get("Authorization") == "" {
			return c.SendStatus(fiber.StatusUnauthorized)
		}
		return c.Next()
	}

	pubApp := fiber.New()
	router.RegisterPublic(pubApp, router.PublicDeps{
		AuthHandler: rejectAuth,
		Topology:    h,
		Subsystem:   subsysteroutes.Config{},
	})

	privApp := fiber.New()
	router.RegisterPrivate(privApp, router.PrivateDeps{
		AuthHandler: rejectAuth,
		Topology:    h,
		Subsystem:   subsysteroutes.Config{},
	})

	for name, app := range map[string]*fiber.App{"public": pubApp, "private": privApp} {
		unauthReq := httptest.NewRequest(http.MethodGet, "/api/v1/topology/history?boardId=b1&windowInterval=60", nil)
		resp, err := app.Test(unauthReq)
		if err != nil {
			t.Fatalf("%s unauth test failed: %v", name, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("%s expected 401 without auth, got %d", name, resp.StatusCode)
		}

		authReq := httptest.NewRequest(http.MethodGet, "/api/v1/topology/history?boardId=b1&windowInterval=60", nil)
		authReq.Header.Set("Authorization", "Bearer test")
		resp2, err := app.Test(authReq)
		if err != nil {
			t.Fatalf("%s auth test failed: %v", name, err)
		}
		resp2.Body.Close()
		if resp2.StatusCode != http.StatusOK {
			t.Fatalf("%s expected 200 with auth, got %d", name, resp2.StatusCode)
		}
	}
}

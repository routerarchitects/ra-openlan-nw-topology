package services

import (
	"context"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/router-architects/ra-openlan-nw-topology/internal/config"
	"github.com/router-architects/ra-openlan-nw-topology/internal/models"
	"github.com/routerarchitects/ow-common-mods/servicerpc/analytics"
	"github.com/routerarchitects/ra-common-mods/apperror"
)

const maxHistoryPartitionCalls = 256

func normalizeHistoryConfig(cfg config.HistoryConfig) config.HistoryConfig {
	def := config.DefaultHistoryConfig()
	if cfg.MaxDurationSeconds == 0 {
		cfg.MaxDurationSeconds = def.MaxDurationSeconds
	}
	if cfg.MaxJitterSeconds == 0 {
		cfg.MaxJitterSeconds = def.MaxJitterSeconds
	}
	if cfg.MaxSnapshotCount == 0 {
		cfg.MaxSnapshotCount = def.MaxSnapshotCount
	}
	if cfg.MaxTimepointsPerFetch <= 0 {
		cfg.MaxTimepointsPerFetch = def.MaxTimepointsPerFetch
	}
	if cfg.MaxTotalTimepoints <= 0 {
		cfg.MaxTotalTimepoints = def.MaxTotalTimepoints
	}
	if cfg.MaxEstimatedOutputUnits == 0 {
		cfg.MaxEstimatedOutputUnits = def.MaxEstimatedOutputUnits
	}
	return cfg
}

type historicalRange struct {
	historyStart int64
	historyEnd   int64
	coveredEnd   int64
	fetchStart   int64
	fetchEnd     int64
	snapshotCnt  uint64
}

// BuildHistoricalTopology builds sliding-window historical topology snapshots for a board.
func (s *topologyService) BuildHistoricalTopology(
	ctx context.Context,
	boardID string,
	duration uint64,
	windowInterval uint64,
	jitter uint64,
) (*models.HistoricalTopologyResponse, error) {
	startedAt := time.Now()
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	boardID = strings.TrimSpace(boardID)
	if err := s.validateHistoricalParams(boardID, duration, windowInterval, jitter); err != nil {
		return nil, err
	}

	nowUnix := s.nowFunc().Unix()
	rng, err := calculateHistoricalRange(nowUnix, duration, windowInterval, jitter)
	if err != nil {
		return nil, err
	}

	members, err := s.fetchBoardMembership(ctx, boardID)
	if err != nil {
		s.logger.Error("failed to fetch board membership for historical topology", "boardId", boardID, "err", err)
		return nil, err
	}

	if rng.snapshotCnt > 0 && uint64(len(members)) > s.historyConfig.MaxEstimatedOutputUnits/rng.snapshotCnt {
		s.logger.Warn("historical topology request exceeds estimated output limit",
			"boardId", boardID,
			"snapshotCount", rng.snapshotCnt,
			"deviceCount", len(members),
			"limit", s.historyConfig.MaxEstimatedOutputUnits,
		)
		return nil, apperror.New(apperror.CodeInvalidInput, "requested historical topology exceeds maximum estimated response size")
	}

	rawRows, err := s.fetchHistoricalTimepoints(ctx, boardID, rng.fetchStart, rng.fetchEnd)
	if err != nil {
		s.logger.Error("failed to fetch historical timepoints",
			"boardId", boardID,
			"fromDate", rng.fetchStart,
			"endDate", rng.fetchEnd,
			"err", err,
		)
		return nil, err
	}

	points, err := prepareHistoricalTimepoints(boardID, rng.fetchStart, rng.fetchEnd, members, rawRows)
	if err != nil {
		s.logger.Error("failed to validate historical timepoints", "boardId", boardID, "err", err)
		return nil, err
	}

	snapshots, err := s.buildHistoricalSnapshots(ctx, rng, windowInterval, jitter, members, points)
	if err != nil {
		return nil, err
	}

	s.logger.Info("historical topology built",
		"boardId", boardID,
		"duration", duration,
		"windowInterval", windowInterval,
		"jitter", jitter,
		"snapshotCount", len(snapshots),
		"timepointCount", len(points),
		"deviceCount", len(members),
		"processingDuration", time.Since(startedAt),
	)

	return &models.HistoricalTopologyResponse{
		BoardID:        boardID,
		FromDate:       rng.historyStart,
		EndDate:        rng.coveredEnd,
		Duration:       duration,
		WindowInterval: windowInterval,
		Jitter:         jitter,
		SnapshotCount:  len(snapshots),
		Snapshots:      snapshots,
	}, nil
}

func (s *topologyService) validateHistoricalParams(
	boardID string,
	duration uint64,
	windowInterval uint64,
	jitter uint64,
) error {
	if boardID == "" {
		return apperror.New(apperror.CodeInvalidInput, "boardId is required")
	}
	if duration == 0 {
		return apperror.New(apperror.CodeInvalidInput, "duration must be greater than 0")
	}
	if windowInterval == 0 {
		return apperror.New(apperror.CodeInvalidInput, "windowInterval must be greater than 0")
	}
	if windowInterval > duration {
		return apperror.New(apperror.CodeInvalidInput, "windowInterval must not exceed duration")
	}
	if duration > s.historyConfig.MaxDurationSeconds {
		return apperror.New(apperror.CodeInvalidInput, fmt.Sprintf("duration exceeds maximum allowed %d seconds", s.historyConfig.MaxDurationSeconds))
	}
	if jitter > s.historyConfig.MaxJitterSeconds {
		return apperror.New(apperror.CodeInvalidInput, fmt.Sprintf("jitter exceeds maximum allowed %d seconds", s.historyConfig.MaxJitterSeconds))
	}
	snapshotCount := duration / windowInterval
	if snapshotCount == 0 {
		return apperror.New(apperror.CodeInvalidInput, "snapshotCount must be greater than 0")
	}
	if snapshotCount > s.historyConfig.MaxSnapshotCount {
		return apperror.New(apperror.CodeInvalidInput, fmt.Sprintf("snapshotCount %d exceeds maximum allowed %d", snapshotCount, s.historyConfig.MaxSnapshotCount))
	}
	return nil
}

func calculateHistoricalRange(nowUnix int64, duration, windowInterval, jitter uint64) (historicalRange, error) {
	if jitter > math.MaxInt64 || duration > math.MaxInt64 {
		return historicalRange{}, apperror.New(apperror.CodeInvalidInput, "timestamp arithmetic overflow")
	}

	jitterI64 := int64(jitter)
	durationI64 := int64(duration)

	if nowUnix <= jitterI64 {
		return historicalRange{}, apperror.New(apperror.CodeInvalidInput, "timestamp underflow calculating historyEnd")
	}
	historyEnd := nowUnix - jitterI64

	if historyEnd <= durationI64 {
		return historicalRange{}, apperror.New(apperror.CodeInvalidInput, "timestamp underflow calculating historyStart")
	}
	historyStart := historyEnd - durationI64

	snapshotCount := duration / windowInterval
	coveredDuration := snapshotCount * windowInterval
	if coveredDuration > math.MaxInt64 {
		return historicalRange{}, apperror.New(apperror.CodeInvalidInput, "timestamp arithmetic overflow calculating coveredEnd")
	}
	coveredEnd := historyStart + int64(coveredDuration)
	if coveredEnd > math.MaxInt64-jitterI64 {
		return historicalRange{}, apperror.New(apperror.CodeInvalidInput, "timestamp arithmetic overflow calculating fetchEnd")
	}

	return historicalRange{
		historyStart: historyStart,
		historyEnd:   historyEnd,
		coveredEnd:   coveredEnd,
		fetchStart:   historyStart,
		fetchEnd:     coveredEnd + jitterI64,
		snapshotCnt:  snapshotCount,
	}, nil
}

func (s *topologyService) fetchBoardMembership(ctx context.Context, boardID string) (map[string]struct{}, error) {
	deviceInfo, err := s.client.GetDeviceInfo(ctx, boardID)
	if err != nil {
		return nil, err
	}

	members := make(map[string]struct{}, len(deviceInfo))
	for _, di := range deviceInfo {
		serial := strings.TrimSpace(di.SerialNumber)
		if serial == "" {
			return nil, apperror.New(apperror.CodeInternal, "malformed device identity: empty serialNumber")
		}
		if diBoard := strings.TrimSpace(di.BoardID); diBoard != "" && diBoard != boardID {
			// Exclude devices whose board identifier does not match the requested board.
			continue
		}
		members[serial] = struct{}{}
	}
	return members, nil
}

type timePartition struct {
	from uint64
	end  uint64
}

func (s *topologyService) fetchHistoricalTimepoints(
	ctx context.Context,
	boardID string,
	fetchStart int64,
	fetchEnd int64,
) ([]analytics.TimepointsData, error) {
	if fetchStart <= 0 || fetchEnd < fetchStart {
		return nil, apperror.New(apperror.CodeInvalidInput, "invalid historical fetch range")
	}

	latestPerDevice := false
	pointsOnly := true
	statsOnly := false

	stack := []timePartition{{from: uint64(fetchStart), end: uint64(fetchEnd)}}
	var allRows []analytics.TimepointsData
	totalFetched := 0
	calls := 0

	for len(stack) > 0 {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		part := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		calls++
		if calls > maxHistoryPartitionCalls {
			return nil, apperror.New(apperror.CodeInvalidInput, "historical timepoint partitioning exceeded maximum call limit")
		}

		rows, err := s.client.GetTimepoints(ctx, analytics.TimepointRequest{
			BoardID:         boardID,
			FromDate:        part.from,
			EndDate:         part.end,
			MaxRecords:      s.historyConfig.MaxTimepointsPerFetch,
			LatestPerDevice: &latestPerDevice,
			PointsOnly:      &pointsOnly,
			StatsOnly:       &statsOnly,
		})
		if err != nil {
			return nil, err
		}

		totalFetched += len(rows)
		if totalFetched > s.historyConfig.MaxTotalTimepoints {
			return nil, apperror.New(apperror.CodeInvalidInput, fmt.Sprintf("historical timepoint count exceeded maximum allowed %d", s.historyConfig.MaxTotalTimepoints))
		}

		if len(rows) < s.historyConfig.MaxTimepointsPerFetch {
			allRows = append(allRows, rows...)
			continue
		}

		// Record limit reached: split the range to guarantee complete retrieval.
		if part.from >= part.end {
			return nil, apperror.New(apperror.CodeInternal, "incomplete historical timepoint retrieval: single-second range reached record limit")
		}

		mid := part.from + (part.end-part.from)/2
		stack = append(stack, timePartition{from: mid + 1, end: part.end})
		stack = append(stack, timePartition{from: part.from, end: mid})
	}

	return allRows, nil
}

func prepareHistoricalTimepoints(
	boardID string,
	fetchStart int64,
	fetchEnd int64,
	members map[string]struct{},
	rawRows []analytics.TimepointsData,
) ([]analytics.TimepointsData, error) {
	seenByID := make(map[string]analytics.TimepointsData, len(rawRows))
	points := make([]analytics.TimepointsData, 0, len(rawRows))

	for _, r := range rawRows {
		topSerial := strings.TrimSpace(r.Serial)
		nestedSerial := strings.TrimSpace(r.DeviceInfo.SerialNumber)
		if topSerial != "" && nestedSerial != "" && topSerial != nestedSerial {
			return nil, apperror.New(apperror.CodeInternal, fmt.Sprintf("conflicting device serial identifiers: %q vs %q", topSerial, nestedSerial))
		}

		serial := resolveSerial(r)
		if serial == "" {
			return nil, apperror.New(apperror.CodeInternal, "malformed timepoint record: missing serial")
		}

		id := strings.TrimSpace(r.ID)
		if id == "" {
			return nil, apperror.New(apperror.CodeInternal, "malformed timepoint record: missing id")
		}

		if rBoard := strings.TrimSpace(r.BoardID); rBoard != "" && rBoard != boardID {
			continue
		}
		if r.Timestamp < fetchStart || r.Timestamp > fetchEnd {
			continue
		}
		if _, ok := members[serial]; !ok {
			continue
		}

		if existing, seen := seenByID[id]; seen {
			if !timepointsPayloadEqual(existing, r) {
				return nil, apperror.New(apperror.CodeConflict, fmt.Sprintf("conflicting timepoint records for id %q", id))
			}
			continue
		}

		seenByID[id] = r
		points = append(points, r)
	}

	sort.Slice(points, func(i, j int) bool {
		if points[i].Timestamp == points[j].Timestamp {
			return points[i].ID < points[j].ID
		}
		return points[i].Timestamp < points[j].Timestamp
	})

	return points, nil
}

func (s *topologyService) buildHistoricalSnapshots(
	ctx context.Context,
	rng historicalRange,
	windowInterval uint64,
	jitter uint64,
	members map[string]struct{},
	points []analytics.TimepointsData,
) ([]models.HistoricalTopologySnapshot, error) {
	latestStates := make(map[string]analytics.TimepointsData, len(members))
	cursor := 0
	snapshots := make([]models.HistoricalTopologySnapshot, 0, rng.snapshotCnt)
	jitterI64 := int64(jitter)
	var totalOutputUnits uint64

	for i := uint64(0); i < rng.snapshotCnt; i++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		windowStart := rng.historyStart + int64(i*windowInterval)
		windowEnd := windowStart + int64(windowInterval)
		effectiveWindowEnd := windowEnd + jitterI64

		for cursor < len(points) && points[cursor].Timestamp <= effectiveWindowEnd {
			point := points[cursor]
			serial := resolveSerial(point)
			if _, exists := members[serial]; exists {
				latestStates[serial] = point
			}
			cursor++
		}

		validStates := make([]analytics.TimepointsData, 0, len(members))
		status := make(map[string]bool, len(members))

		for serial := range members {
			point, exists := latestStates[serial]
			if exists && point.Timestamp >= windowStart {
				status[serial] = true
				validStates = append(validStates, point)
			} else {
				delete(latestStates, serial)
				status[serial] = false
			}
		}

		// Order descending by timestamp and ID so buildDevicesAndMeshEdges keeps the latest
		// client association observation when a client roams across APs in the same window.
		sort.Slice(validStates, func(a, b int) bool {
			if validStates[a].Timestamp == validStates[b].Timestamp {
				if validStates[a].ID == validStates[b].ID {
					return resolveSerial(validStates[a]) < resolveSerial(validStates[b])
				}
				return validStates[a].ID > validStates[b].ID
			}
			return validStates[a].Timestamp > validStates[b].Timestamp
		})

		devs, meshEdges := s.buildTopologyFromStates(validStates, status)
		histDevs := toHistoricalDevices(devs)

		totalOutputUnits += countSnapshotOutputUnits(histDevs, meshEdges)
		if totalOutputUnits > s.historyConfig.MaxEstimatedOutputUnits {
			s.logger.Warn("historical topology output exceeded maximum allowed units",
				"totalOutputUnits", totalOutputUnits,
				"limit", s.historyConfig.MaxEstimatedOutputUnits,
				"snapshotIndex", i,
			)
			return nil, apperror.New(apperror.CodeInvalidInput, fmt.Sprintf("historical topology response size exceeds maximum allowed %d output units", s.historyConfig.MaxEstimatedOutputUnits))
		}

		snapshots = append(snapshots, models.HistoricalTopologySnapshot{
			Timestamp:          windowEnd,
			WindowStart:        windowStart,
			WindowEnd:          windowEnd,
			EffectiveWindowEnd: effectiveWindowEnd,
			Nodes:              histDevs,
			Edges: models.TopoEdges{
				Wired: []any{},
				Mesh:  meshEdges,
			},
		})
	}

	return snapshots, nil
}

func toHistoricalDevices(devs []models.Device) []models.HistoricalDevice {
	out := make([]models.HistoricalDevice, 0, len(devs))
	for _, d := range devs {
		aps := d.APs
		if aps == nil {
			aps = []models.Iface{}
		}
		mesh := d.Mesh
		if mesh == nil {
			mesh = []models.Iface{}
		}
		out = append(out, models.HistoricalDevice{
			Serial:    d.Serial,
			Uptime:    d.Uptime,
			APs:       aps,
			Mesh:      mesh,
			Connected: d.Connected,
		})
	}
	return out
}

func countSnapshotOutputUnits(nodes []models.HistoricalDevice, meshEdges []models.MeshEdge) uint64 {
	units := uint64(len(nodes) + len(meshEdges))
	for _, n := range nodes {
		units += uint64(len(n.APs) + len(n.Mesh))
		for _, iface := range n.APs {
			units += uint64(len(iface.Clients))
		}
		for _, iface := range n.Mesh {
			units += uint64(len(iface.Clients))
		}
	}
	return units
}

func timepointsPayloadEqual(a, b analytics.TimepointsData) bool {
	if a.Timestamp != b.Timestamp {
		return false
	}
	if strings.TrimSpace(a.BoardID) != strings.TrimSpace(b.BoardID) {
		return false
	}
	if resolveSerial(a) != resolveSerial(b) {
		return false
	}
	return reflect.DeepEqual(a.DeviceInfo, b.DeviceInfo) && reflect.DeepEqual(a.SSIDData, b.SSIDData)
}

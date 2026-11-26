package models

// HistoricalTopologyQuery represents the validated query parameters for GET /api/v1/topology/history.
type HistoricalTopologyQuery struct {
	BoardID        string  `query:"boardId"`
	Duration       *uint64 `query:"duration"`
	WindowInterval *uint64 `query:"windowInterval"`
	Jitter         *uint64 `query:"jitter"`
}

// HistoricalTopologyResponse is the response contract for GET /api/v1/topology/history.
type HistoricalTopologyResponse struct {
	BoardID        string                       `json:"boardId"`
	FromDate       int64                        `json:"fromDate"`
	EndDate        int64                        `json:"endDate"`
	Duration       uint64                       `json:"duration"`
	WindowInterval uint64                       `json:"windowInterval"`
	Jitter         uint64                       `json:"jitter"`
	SnapshotCount  int                          `json:"snapshotCount"`
	Snapshots      []HistoricalTopologySnapshot `json:"snapshots"`
}

// HistoricalTopologySnapshot represents a single sliding-window topology snapshot.
type HistoricalTopologySnapshot struct {
	Timestamp          int64              `json:"timestamp"`
	WindowStart        int64              `json:"windowStart"`
	WindowEnd          int64              `json:"windowEnd"`
	EffectiveWindowEnd int64              `json:"effectiveWindowEnd"`
	Nodes              []HistoricalDevice `json:"nodes"`
	Edges              TopoEdges          `json:"edges"`
}

// HistoricalDevice represents a device node in a historical topology snapshot.
// Unlike Device, aps and mesh omit omitempty so offline or empty nodes serialize explicit [].
type HistoricalDevice struct {
	Serial    string  `json:"serial"`
	Uptime    int64   `json:"uptime,omitempty"`
	APs       []Iface `json:"aps"`
	Mesh      []Iface `json:"mesh"`
	Connected bool    `json:"connected"`
}

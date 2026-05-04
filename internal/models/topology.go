package models

// Topology is the response model shaped per your example.
type Topology struct {
	BoardID           string    `json:"boardId,omitempty"`
	Timestamp         string    `json:"timestamp,omitempty"` // RFC3339 UTC
	Nodes             []Device  `json:"nodes,omitempty"`
	HistoricalDevices []string  `json:"historicalDevices,omitempty"` // new field for historical client MACs
	Edges             TopoEdges `json:"edges,omitempty"`
	External          []any     `json:"external,omitempty"`
}

// Device groups interfaces by device serial.
type Device struct {
	Serial    string  `json:"serial,omitempty"`
	Uptime    int64   `json:"uptime,omitempty"`
	APs       []Iface `json:"aps,omitempty"`
	Mesh      []Iface `json:"mesh,omitempty"`
	Connected bool    `json:"connected"`
}

// Iface (interface) represents one BSSID on a band (AP or Mesh) at a specific sample timestamp.
type Iface struct {
	BSSID   string        `json:"bssid"`
	SSID    string        `json:"ssid"`
	Band    string        `json:"band"` // "2" | "5" | "6" etc. (string per your sample)
	Channel int           `json:"channel"`
	Mode    string        `json:"mode"`    // "ap" | "mesh"
	Clients []IfaceClient `json:"clients"` // nil -> JSON null
}

// IfaceClient is an association record on a Iface at that timestamp.
type IfaceClient struct {
	Station       string `json:"station"` // MAC (normalized)
	RSSI          int    `json:"rssi"`
	Connected     int    `json:"connected"`
	Inactive      int    `json:"inactive"`
	RxRateBitrate int    `json:"rx_rate_bitrate"`
	TxRateBitrate int    `json:"tx_rate_bitrate"`
	RxRateChwidth int    `json:"rx_rate_chwidth"`
	Fingerprint   string `json:"fingerprint,omitempty"`
}

type TopoEdges struct {
	Wired []any      `json:"wired"` // kept for future; empty now
	Mesh  []MeshEdge `json:"mesh"`
}

// MeshEdge is a directed edge from one device (serial) to another via mesh.
type MeshEdge struct {
	From    string `json:"from"` // serial
	To      string `json:"to"`   // serial
	SSID    string `json:"ssid"`
	Band    string `json:"band"`
	Channel int    `json:"channel"`
}

type TimepointsQuery struct {
	BoardID  string  `query:"boardId" validate:"required,notblank"`
	Interval *uint64 `query:"interval" validate:"omitempty,gt=1,lte=86400"`
}

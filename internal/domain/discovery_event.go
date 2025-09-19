package domain

// DiscoveryEvent mirrors the JSON payload published for service discovery updates.
type DiscoveryEvent struct {
    Event           string `json:"event"`
    ID              int64  `json:"id"`
    Key             string `json:"key"`
    PrivateEndPoint string `json:"privateEndPoint"`
    PublicEndPoint  string `json:"publicEndPoint"`
    Type            string `json:"type"`
    Version         string `json:"version"`
}

package models

type SetSubsytemPayload struct {
	Subsystems []struct {
		Tag   string `json:"tag"`
		Value string `json:"value"`
	} `json:"subsystems"`
}

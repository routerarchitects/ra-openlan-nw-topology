package models

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// Outbound message to CGW
type KafkaCommand struct {
	UUID    string                 `json:"uuid"`
	Type    string                 `json:"type"`
	Payload map[string]interface{} `json:"payload"`
}

// Inbound response from CGW
type KafkaResponse struct {
	UUID            string         `json:"uuid"`
	ReporterShardID *int           `json:"reporter_shard_id,omitempty"`
	Type            string         `json:"type,omitempty"`
	Success         bool           `json:"success,omitempty"`
	ErrorMsg        string         `json:"error_message,omitempty"`
	Payload         map[string]any `json:"-"`
}

func (kr *KafkaResponse) UnmarshalJSON(data []byte) error {
	// 1) Unmarshal into raw map so we can peel off known keys.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("decode raw: %w", err)
	}

	// 2) Known fields
	if rm, ok := raw["success"]; ok {
		if err := json.Unmarshal(rm, &kr.Success); err != nil {
			return fmt.Errorf("decode success: %w", err)
		}
		delete(raw, "success")
	}
	if rm, ok := raw["error_message"]; ok {
		if err := json.Unmarshal(rm, &kr.ErrorMsg); err != nil {
			return fmt.Errorf("decode error_message: %w", err)
		}
		delete(raw, "error_message")
	}
	if rm, ok := raw["uuid"]; ok {
		if err := json.Unmarshal(rm, &kr.UUID); err != nil {
			return fmt.Errorf("decode uuid: %w", err)
		}
		delete(raw, "uuid")
	}
	if rm, ok := raw["type"]; ok {
		if err := json.Unmarshal(rm, &kr.Type); err != nil {
			return fmt.Errorf("decode type: %w", err)
		}
		delete(raw, "type")
	}
	if rm, ok := raw["reporter_shard_id"]; ok {
		if err := json.Unmarshal(rm, &kr.ReporterShardID); err != nil {
			return fmt.Errorf("decode type: %w", err)
		}
		delete(raw, "reporter_shard_id")
	}

	// 3) Everything else → Other (numbers preserved as ints when possible)
	kr.Payload = make(map[string]any, len(raw))
	for k, rm := range raw {
		dec := json.NewDecoder(bytes.NewReader(rm))
		dec.UseNumber() // keep numbers as json.Number so we can decide int/float
		var v any
		if err := dec.Decode(&v); err != nil {
			return fmt.Errorf("decode other[%s]: %w", k, err)
		}
		kr.Payload[k] = v
	}
	return nil
}

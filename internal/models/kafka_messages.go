package models

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/router-architects/network-topology-service/internal/apperrors"
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
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return apperrors.WrapError(apperrors.CodeInvalidInput, "decode raw", err)
	}

	if rm, ok := raw["success"]; ok {
		if err := json.Unmarshal(rm, &kr.Success); err != nil {
			return apperrors.WrapError(apperrors.CodeInvalidInput, "decode success", err)
		}
		delete(raw, "success")
	}
	if rm, ok := raw["error_message"]; ok {
		if err := json.Unmarshal(rm, &kr.ErrorMsg); err != nil {
			return apperrors.WrapError(apperrors.CodeInvalidInput, "decode error_message", err)
		}
		delete(raw, "error_message")
	}
	if rm, ok := raw["uuid"]; ok {
		if err := json.Unmarshal(rm, &kr.UUID); err != nil {
			return apperrors.WrapError(apperrors.CodeInvalidInput, "decode uuid", err)
		}
		delete(raw, "uuid")
	}
	if rm, ok := raw["type"]; ok {
		if err := json.Unmarshal(rm, &kr.Type); err != nil {
			return apperrors.WrapError(apperrors.CodeInvalidInput, "decode type", err)
		}
		delete(raw, "type")
	}
	if rm, ok := raw["reporter_shard_id"]; ok {
		if err := json.Unmarshal(rm, &kr.ReporterShardID); err != nil {
			return apperrors.WrapError(apperrors.CodeInvalidInput, "decode reporter_shard_id", err)
		}
		delete(raw, "reporter_shard_id")
	}

	kr.Payload = make(map[string]any, len(raw))
	for k, rm := range raw {
		dec := json.NewDecoder(bytes.NewReader(rm))
		dec.UseNumber()
		var v any
		if err := dec.Decode(&v); err != nil {
			return apperrors.WrapError(apperrors.CodeInvalidInput, fmt.Sprintf("decode other[%s]", k), err)
		}
		kr.Payload[k] = v
	}
	return nil
}

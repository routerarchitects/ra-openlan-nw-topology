package utils

import (
	"crypto/rand"
	"errors"
	"fmt"
	"strconv"
	"time"
)

func ParseRFC3339OrUTC(s string) (time.Time, error) {
	// If user omits zone, treat as UTC.
	// time.Parse requires a zone to be present; we attempt to append 'Z' if missing.
	if s == "" {
		return time.Time{}, nil
	}
	if _, err := time.Parse(time.RFC3339, s); err == nil {
		t, _ := time.Parse(time.RFC3339, s)
		return t.UTC(), nil
	}
	// try append 'Z' if looks like "YYYY-MM-DDTHH:MM:SS"
	if _, err := time.Parse("2006-01-02T15:04:05", s); err == nil {
		t, _ := time.Parse("2006-01-02T15:04:05", s)
		return t.UTC(), nil
	}
	return time.Time{}, errors.New("InvalidTimeFormat") // set by handlers
}

func EpochSeconds(t time.Time) int64 { return t.UTC().Unix() }

func FromEpochSeconds(sec int64) time.Time { return time.Unix(sec, 0).UTC() }

// For defensive parsing of epoch query (if ever added).
func ParseEpoch(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}

func NewUUID() (string, error) {
	// uuid v4 (fast-n-simple)
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("uuid: %w", err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

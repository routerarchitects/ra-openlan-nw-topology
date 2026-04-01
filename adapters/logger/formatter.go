package logger

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
)

// LegacyFormatter renders logs in the `timestamp | threadName : [Level][thr:x] msg k=v` layout.
type LegacyFormatter struct {
	TimestampFormat string
	ThreadID        int
}

func (f *LegacyFormatter) Format(entry *logrus.Entry) ([]byte, error) {

	threadID := f.ThreadID
	if v, ok := entry.Data["thread"]; ok {
		if id, err := strconv.Atoi(fmt.Sprint(v)); err == nil {
			threadID = id
		}
	}

	tsLayout := f.TimestampFormat
	if tsLayout == "" {
		tsLayout = "2006-01-02 15:04:05.000"
	}
	timestamp := entry.Time.Format(tsLayout)

	levelLabel := legacyLevel(entry.Level)
	msg := entry.Message

	threadName := strings.TrimSpace(getStringField(entry.Data, "threadName"))
	fields := serializeFields(entry.Data, []string{"component", "service", "thread", "threadName"})

	var buf bytes.Buffer
	if threadName != "" {
		fmt.Fprintf(&buf, "%s | %s : [%s][thr:%d] %s", timestamp, threadName, levelLabel, threadID, msg)
	} else {
		fmt.Fprintf(&buf, "%s : [%s][thr:%d] %s", timestamp, levelLabel, threadID, msg)
	}
	if len(fields) > 0 {
		buf.WriteByte(' ')
		buf.WriteString(fields)
	}
	buf.WriteByte('\n')
	return buf.Bytes(), nil
}

func getStringField(fields logrus.Fields, keys ...string) string {
	for _, k := range keys {
		if v, ok := fields[k]; ok {
			return fmt.Sprint(v)
		}
	}
	return ""
}

func legacyLevel(level logrus.Level) string {
	switch level {
	case logrus.TraceLevel:
		return "Trace"
	case logrus.DebugLevel:
		return "Debug"
	case logrus.InfoLevel:
		return "Information"
	case logrus.WarnLevel:
		return "Notice"
	case logrus.ErrorLevel:
		return "Error"
	case logrus.FatalLevel:
		return "Fatal"
	default:
		return level.String()
	}
}

func serializeFields(fields logrus.Fields, ignored []string) string {
	ignoreSet := make(map[string]struct{}, len(ignored))
	for _, key := range ignored {
		ignoreSet[key] = struct{}{}
	}

	keys := make([]string, 0, len(fields))
	for k := range fields {
		if _, skip := ignoreSet[k]; skip {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	if len(keys) == 0 {
		return ""
	}

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%v", k, fields[k]))
	}
	return strings.Join(parts, " ")
}

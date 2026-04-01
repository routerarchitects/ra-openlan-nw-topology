package logger

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

type Fields map[string]interface{}

type Logger interface {
	Trace(args ...interface{})
	Debug(args ...interface{})
	Info(args ...interface{})
	Warn(args ...interface{})
	Error(args ...interface{})
	Fatal(args ...interface{})

	WithFields(fields Fields) Logger
	WithField(key string, value interface{}) Logger
	WithError(err error) Logger
}

type stdLogger struct {
	base       *logrus.Entry
	fields     Fields
	json       bool
	timeFormat string
}

var globalLogger Logger

func New(json bool, timeFormat string, level string) Logger {
	if timeFormat == "" {
		timeFormat = time.RFC3339
	}

	log := logrus.New()
	log.SetOutput(os.Stdout)

	logLevel, err := logrus.ParseLevel(level)
	if err != nil {
		logrus.Errorf("GetLogger level is incorrect [%v]. Setting default to Info", level)
		logLevel = logrus.InfoLevel
	}
	log.SetLevel(logLevel)
	log.SetReportCaller(false)

	return &stdLogger{
		base:       logrus.NewEntry(log),
		fields:     Fields{},
		json:       json,
		timeFormat: timeFormat,
	}
}

func SetLogger(l Logger) { globalLogger = l }
func GetLogger() Logger  { return globalLogger }



func (l *stdLogger) cloneWith(add Fields) *stdLogger {
	out := make(Fields, len(l.fields)+len(add))
	for k, v := range l.fields {
		out[k] = v
	}
	for k, v := range add {
		out[k] = v
	}
	return &stdLogger{
		base:       l.base,
		fields:     out,
		json:       l.json,
		timeFormat: l.timeFormat,
	}
}

func (l *stdLogger) WithFields(fields Fields) Logger {
	return l.cloneWith(fields)
}

func (l *stdLogger) WithField(key string, value interface{}) Logger {
	return l.cloneWith(Fields{key: value})
}

func (l *stdLogger) WithError(err error) Logger {
	return l.WithField("error", err)
}

func (l *stdLogger) Trace(a ...interface{}) { l.log("TRACE", fmt.Sprint(a...)) }
func (l *stdLogger) Debug(a ...interface{}) { l.log("DEBUG", fmt.Sprint(a...)) }
func (l *stdLogger) Info(a ...interface{})  { l.log("INFO", fmt.Sprint(a...)) }
func (l *stdLogger) Warn(a ...interface{})  { l.log("WARN", fmt.Sprint(a...)) }
func (l *stdLogger) Error(a ...interface{}) { l.log("ERROR", fmt.Sprint(a...)) }
func (l *stdLogger) Fatal(a ...interface{}) { l.log("FATAL", fmt.Sprint(a...)); os.Exit(1) }

func (l *stdLogger) log(level, msg string) {
	now := time.Now().Format(l.timeFormat)

	if l.json {
		m := make(map[string]interface{}, len(l.fields)+3)
		for k, v := range l.fields {
			m[k] = v
		}
		m["time"] = now
		m["level"] = level
		m["msg"] = msg

		b, err := json.Marshal(m)
		if err != nil {
			l.base.Println(msg)
			return
		}
		l.base.Println(string(b))
		return
	}

	var functionality string
	other := make(Fields)
	for k, v := range l.fields {
		if k == "threadName" {
			if s, ok := v.(string); ok {
				functionality = strings.TrimSpace(s)
				continue
			}
		}
		other[k] = v
	}

	keys := make([]string, 0, len(other))
	for k := range other {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%v", k, other[k]))
	}
	fields := strings.Join(parts, " ")

	if functionality != "" {
		if fields != "" {
			l.base.Printf("%s %s : [%s] %s %s\n", now, functionality, level, msg, fields)
		} else {
			l.base.Printf("%s %s : [%s] %s\n", now, functionality, level, msg)
		}
	} else {
		if fields != "" {
			l.base.Printf("%s : [%s] %s %s\n", now, level, msg, fields)
		} else {
			l.base.Printf("%s : [%s] %s\n", now, level, msg)
		}
	}
}

package logger

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/sirupsen/logrus"
)

// LogrusLogger is a wrapper around logrus.Logger to conform to the Logger interface.
type LogrusLogger struct {
	logger *logrus.Entry
}

const (
	// TimestampFormat defines the format for timestamps in log entries.
	TimestampFormat = "2006-01-02 15:04:05"
)

// NewLogrusLogger initializes a new LogrusLogger.
func NewLogrusLogger(level string) *LogrusLogger {

	log := logrus.New()
	log.SetFormatter(&LegacyFormatter{
		TimestampFormat: TimestampFormat,
	})
	log.SetOutput(os.Stdout)
	entry := logrus.NewEntry(log)

	logLevel, err := logrus.ParseLevel(level)
	if err != nil {
		logrus.Errorf("GetLogger level is incorrect [%v]. Setting default to Info", level)
		// logLevel = logrus.InfoLevel
	}
	log.SetLevel(logLevel)
	log.SetReportCaller(false)
	return &LogrusLogger{logger: entry}
}

func (l *LogrusLogger) Trace(args ...interface{}) {
	if !l.allowedBySubsystem(logrus.TraceLevel) {
		return
	}
	l.logger.Trace(args...)
}

func (l *LogrusLogger) Debug(args ...interface{}) {
	if !l.allowedBySubsystem(logrus.DebugLevel) {
		return
	}
	l.logger.Debug(args...)
}

func (l *LogrusLogger) Info(args ...interface{}) {
	if !l.allowedBySubsystem(logrus.InfoLevel) {
		return
	}
	l.logger.Info(args...)
}

func (l *LogrusLogger) Warn(args ...interface{}) {
	if !l.allowedBySubsystem(logrus.WarnLevel) {
		return
	}

	l.logger.WithFields(logrus.Fields{
		"file":     getCallerFile(2), // Increase skip level to bypass wrapper
		"function": getFuncName(2),
		"line":     getCallerLine(2),
	}).Warn(args...)
}

func (l *LogrusLogger) Error(args ...interface{}) {
	if !l.allowedBySubsystem(logrus.ErrorLevel) {
		return
	}

	l.logger.WithFields(logrus.Fields{
		"file":     getCallerFile(2), // Increase skip level to bypass wrapper
		"function": getFuncName(2),
		"line":     getCallerLine(2),
	}).Error(args...)
}

func (l *LogrusLogger) Fatal(args ...interface{}) {
	if !l.allowedBySubsystem(logrus.FatalLevel) {
		return
	}

	l.logger.WithFields(logrus.Fields{
		"file":     getCallerFile(2), // Increase skip level to bypass wrapper
		"function": getFuncName(2),
		"line":     getCallerLine(2),
	}).Fatal(args...)
}

// Formatted methods

func (l *LogrusLogger) Tracef(format string, args ...interface{}) {
	if !l.allowedBySubsystem(logrus.TraceLevel) {
		return
	}
	l.logger.Tracef(format, args...)
}

func (l *LogrusLogger) Debugf(format string, args ...interface{}) {
	if !l.allowedBySubsystem(logrus.DebugLevel) {
		return
	}
	l.logger.Debugf(format, args...)
}

func (l *LogrusLogger) Infof(format string, args ...interface{}) {
	if !l.allowedBySubsystem(logrus.InfoLevel) {
		return
	}
	l.logger.Infof(format, args...)
}

func (l *LogrusLogger) Warnf(format string, args ...interface{}) {
	if !l.allowedBySubsystem(logrus.WarnLevel) {
		return
	}

	l.logger.WithFields(logrus.Fields{
		"file":     getCallerFile(2), // Increase skip level to bypass wrapper
		"function": getFuncName(2),
		"line":     getCallerLine(2),
	}).Warnf(format, args...)
}

func (l *LogrusLogger) Errorf(format string, args ...interface{}) {
	if !l.allowedBySubsystem(logrus.ErrorLevel) {
		return
	}

	l.logger.WithFields(logrus.Fields{
		"file":     getCallerFile(2), // Increase skip level to bypass wrapper
		"function": getFuncName(2),
		"line":     getCallerLine(2),
	}).Errorf(format, args...)
}

func (l *LogrusLogger) Fatalf(format string, args ...interface{}) {
	if !l.allowedBySubsystem(logrus.FatalLevel) {
		return
	}

	l.logger.WithFields(logrus.Fields{
		"file":     getCallerFile(2), // Increase skip level to bypass wrapper
		"function": getFuncName(2),
		"line":     getCallerLine(2),
	}).Fatalf(format, args...)
}

func (l *LogrusLogger) allowedBySubsystem(level logrus.Level) bool {
	if l == nil || l.logger == nil {
		return true
	}
	if l.logger.Data == nil {
		return true
	}
	raw, ok := l.logger.Data["threadName"]
	if !ok {
		return true
	}
	threadName, ok := raw.(string)
	if !ok || threadName == "" {
		return true
	}
	minLevel := GetSubsystemLevel(threadName)
	return level <= minLevel
}

func (l *LogrusLogger) WithFields(fields Fields) Logger {
	return &LogrusLogger{logger: l.logger.WithFields(logrus.Fields(fields))}
}

func (l *LogrusLogger) WithField(key string, value interface{}) Logger {
	return &LogrusLogger{logger: l.logger.WithField(key, value)}
}

func (l *LogrusLogger) WithError(err error) Logger {
	return &LogrusLogger{logger: l.logger.WithError(err)}
}

func getCallerFile(skip int) string {
	_, file, _, ok := runtime.Caller(skip)
	if !ok {
		return "unknown"
	}
	return filepath.Base(file)
}

// Custom function to get caller line
func getCallerLine(skip int) int {
	_, _, line, ok := runtime.Caller(skip)
	if !ok {
		return 0
	}
	return line
}

func getFuncName(skip int) string {
	pc, _, _, ok := runtime.Caller(skip)
	if !ok {
		return "Unknown"
	}
	return trimFuncName(runtime.FuncForPC(pc).Name())
}

// trimFuncName extracts only the package and function name.
func trimFuncName(fullFuncName string) string {
	// Split the full function name by '/' to remove the path
	parts := strings.Split(fullFuncName, "/")
	lastPart := parts[len(parts)-1] // Get the last part after the last '/'

	// Now split by '.' to separate package and function parts
	pkgFuncParts := strings.SplitN(lastPart, ".", 2)
	if len(pkgFuncParts) > 1 {
		return pkgFuncParts[0] + "." + pkgFuncParts[1]
	}
	return lastPart
}

// Fields is a convenience alias for structured log fields.
type Fields map[string]interface{}

// Logger defines the minimal interface our app expects from a logger implementation.
// (We adapt logrus to this interface in cmd/main.go.)
type Logger interface {
	Trace(args ...interface{})
	Debug(args ...interface{})
	Info(args ...interface{})
	Warn(args ...interface{})
	Error(args ...interface{})
	Fatal(args ...interface{})

	Tracef(format string, args ...interface{})
	Debugf(format string, args ...interface{})
	Infof(format string, args ...interface{})
	Warnf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
	Fatalf(format string, args ...interface{})

	WithFields(fields Fields) Logger
	WithField(key string, value interface{}) Logger
	WithError(err error) Logger
}

var globalLogger Logger

// SetLogger sets the global logger instance used by the app.
func SetLogger(l Logger) { globalLogger = l }

// GetLogger gets the global logger instance (may be nil if not initialized).
func GetLogger() Logger { return globalLogger }

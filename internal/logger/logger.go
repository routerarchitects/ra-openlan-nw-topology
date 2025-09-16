package logger

import "github.com/sirupsen/logrus"

type LogrusAdapter struct{ *logrus.Entry }

func (l LogrusAdapter) Trace(args ...interface{})         { l.Entry.Trace(args...) }
func (l LogrusAdapter) Debug(args ...interface{})         { l.Entry.Debug(args...) }
func (l LogrusAdapter) Info(args ...interface{})          { l.Entry.Info(args...) }
func (l LogrusAdapter) Warn(args ...interface{})          { l.Entry.Warn(args...) }
func (l LogrusAdapter) Error(args ...interface{})         { l.Entry.Error(args...) }
func (l LogrusAdapter) Fatal(args ...interface{})         { l.Entry.Fatal(args...) }
func (l LogrusAdapter) Tracef(f string, a ...interface{}) { l.Entry.Tracef(f, a...) }
func (l LogrusAdapter) Debugf(f string, a ...interface{}) { l.Entry.Debugf(f, a...) }
func (l LogrusAdapter) Infof(f string, a ...interface{})  { l.Entry.Infof(f, a...) }
func (l LogrusAdapter) Warnf(f string, a ...interface{})  { l.Entry.Warnf(f, a...) }
func (l LogrusAdapter) Errorf(f string, a ...interface{}) { l.Entry.Errorf(f, a...) }
func (l LogrusAdapter) Fatalf(f string, a ...interface{}) { l.Entry.Fatalf(f, a...) }
func (l LogrusAdapter) WithFields(fields Fields) Logger {
	return LogrusAdapter{l.Entry.WithFields(logrus.Fields(fields))}
}
func (l LogrusAdapter) WithField(key string, value interface{}) Logger {
	return LogrusAdapter{l.Entry.WithField(key, value)}
}
func (l LogrusAdapter) WithError(err error) Logger {
	return LogrusAdapter{l.Entry.WithError(err)}
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

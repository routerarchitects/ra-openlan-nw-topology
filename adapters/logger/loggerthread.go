package logger

const (
	ThreadKafkaConsumer = "KAFKA-CONSUMER"
	ThreadDiscovery     = "DISCOVERY"
	ThreadServer        = "SERVER"
)

func threadIDForName(name string) int {
	switch name {
	case ThreadKafkaConsumer:
		return 1
	case ThreadDiscovery:
		return 2
	case ThreadServer:
		return 3
	default:
		return 0 // fallback / unknown
	}
}

// GetLoggerThreadId returns a logger tagged with threadName + thread (if present in ctx).
func GetLoggerThreadId(threadName string) Logger {
	log := GetLogger()
	if log == nil {
		return nil
	}

	id := threadIDForName(threadName)

	return log.WithFields(Fields{
		"threadName": threadName,
		"thread":     id,
	})
}

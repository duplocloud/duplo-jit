package duplocloud

import "log"

const (
	FATAL = 0
	ERROR = 1
	WARN  = 2
	INFO  = 3
	DEBUG = 4
	TRACE = 5
)

var LogLevel = 1

func logf(level int, msg string, v ...interface{}) {
	if level <= LogLevel {
		log.Printf(msg, v...)
	}
}

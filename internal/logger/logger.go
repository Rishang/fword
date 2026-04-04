// Package logger wraps charmbracelet/log with a project-level default logger.
// Call SetDebug(true) to enable debug output; all other levels are always active.
package logger

import (
	"os"

	chlog "github.com/charmbracelet/log"
)

var log = chlog.NewWithOptions(os.Stderr, chlog.Options{
	Prefix: "fk",
	Level:  chlog.InfoLevel,
})

// SetDebug drops the log level to Debug so debug messages are printed.
func SetDebug(v bool) {
	if v {
		log.SetLevel(chlog.DebugLevel)
	} else {
		log.SetLevel(chlog.InfoLevel)
	}
}

func Debug(msg string, keyvals ...any) { log.Debug(msg, keyvals...) }
func Info(msg string, keyvals ...any)  { log.Info(msg, keyvals...) }
func Warn(msg string, keyvals ...any)  { log.Warn(msg, keyvals...) }
func Error(msg string, keyvals ...any) { log.Error(msg, keyvals...) }

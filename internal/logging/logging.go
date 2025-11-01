package logging

import (
	"fmt"
	"os"
	"strings"
	"time"
)

var enabled bool

// SetDebug enables or disables debug logging globally.
func SetDebug(on bool) { enabled = on }

// Enabled reports whether debug logging is enabled.
func Enabled() bool { return enabled }

// Debugf prints a formatted debug message to stderr when enabled.
func Debugf(format string, args ...any) {
	if !enabled {
		return
	}
	ts := time.Now().Format(time.RFC3339)
	fmt.Fprintf(os.Stderr, "[DEBUG] %s %s\n", ts, fmt.Sprintf(format, args...))
}

// Debug prints a debug message to stderr when enabled.
func Debug(args ...any) {
	if !enabled {
		return
	}
	ts := time.Now().Format(time.RFC3339)
	msg := strings.TrimSuffix(fmt.Sprintln(args...), "\n")
	fmt.Fprintf(os.Stderr, "[DEBUG] %s %s\n", ts, msg)
}

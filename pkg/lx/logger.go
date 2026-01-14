package lx

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

type LogLevel int

const (
	LevelSilent LogLevel = iota
	LevelError
	LevelWarn
	LevelInfo
	LevelDebug
	LevelTrace
)

type Logger struct {
	out   io.Writer
	level LogLevel
}

func NewLogger(out io.Writer, level LogLevel) *Logger {
	if out == nil {
		out = os.Stderr
	}
	return &Logger{out: out, level: level}
}

func ParseLevel(s string) LogLevel {
	switch strings.ToLower(s) {
	case "trace":
		return LevelTrace
	case "debug":
		return LevelDebug
	case "info":
		return LevelInfo
	case "warn", "warning":
		return LevelWarn
	case "error":
		return LevelError
	case "silent", "quiet", "off":
		return LevelSilent
	default:
		// Default to Info if unknown string.
		return LevelInfo
	}
}

func (l *Logger) Errorf(format string, a ...interface{}) {
	if l.level >= LevelError {
		l.print("ERROR", format, a...)
	}
}

func (l *Logger) Warnf(format string, a ...interface{}) {
	if l.level >= LevelWarn {
		l.print("WARN", format, a...)
	}
}

func (l *Logger) Infof(format string, a ...interface{}) {
	if l.level >= LevelInfo {
		l.print("INFO", format, a...)
	}
}

func (l *Logger) Debugf(format string, a ...interface{}) {
	if l.level >= LevelDebug {
		l.print("DEBUG", format, a...)
	}
}

func (l *Logger) Tracef(format string, a ...interface{}) {
	if l.level >= LevelTrace {
		l.print("TRACE", format, a...)
	}
}

func (l *Logger) print(level string, format string, a ...interface{}) {
	// Format: 15:04:05.000 LEVEL : Message
	ts := time.Now().Format("15:04:05.000000")
	msg := fmt.Sprintf(format, a...)
	fmt.Fprintf(l.out, "%s %-5s : %s\n", ts, level, msg)
}

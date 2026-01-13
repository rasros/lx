package lx

import (
	"fmt"
	"io"
	"os"
	"strings"
)

type LogLevel int

const (
	LevelSilent LogLevel = iota
	LevelError
	LevelWarn
	LevelInfo
	LevelDebug
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
		fmt.Fprintf(l.out, "ERROR: "+format+"\n", a...)
	}
}

func (l *Logger) Warnf(format string, a ...interface{}) {
	if l.level >= LevelWarn {
		fmt.Fprintf(l.out, "WARN:  "+format+"\n", a...)
	}
}

func (l *Logger) Infof(format string, a ...interface{}) {
	if l.level >= LevelInfo {
		fmt.Fprintf(l.out, "INFO:  "+format+"\n", a...)
	}
}

func (l *Logger) Debugf(format string, a ...interface{}) {
	if l.level >= LevelDebug {
		fmt.Fprintf(l.out, "DEBUG: "+format+"\n", a...)
	}
}

package types

import (
	"fmt"
	"io"
	"os"
	"time"
)

type LoggerI interface {
	// Debug prints a debug message.
	Debug(msg string)

	// Info prints an info message.
	Info(msg string)

	// Warn prints a warning message.
	Warn(msg string)

	// Error prints an error message.
	Error(msg string)

	// Print prints a message with no level.
	Print(msg string)

	// Debugf prints a debug message with formatting.
	Debugf(format string, args ...interface{})

	// Infof prints an info message with formatting.
	Infof(format string, args ...interface{})

	// Warnf prints a warning message with formatting.
	Warnf(format string, args ...interface{})

	// Errorf prints an error message with formatting.
	Errorf(format string, args ...interface{})

	// Printf prints a message with no level and formatting.
	Printf(format string, args ...interface{})
}

const (
	// DebugLevel is the debug level.
	DebugLevel int32 = -4
	// InfoLevel is the info level.
	InfoLevel int32 = 0
	// WarnLevel is the warn level.
	WarnLevel int32 = 4
	// ErrorLevel is the error level.
	ErrorLevel int32 = 8

	Reset  = "\033[0m"
	RED    = "\033[31m"
	GREEN  = "\033[32m"
	YELLOW = "\033[33m"
	BLUE   = "\033[34m"
	PURPLE = "\033[35m"
	GRAY   = "\033[37m"
)

var (
	_ LoggerI = &Logger{}

	DefaultLoggerConfig = LoggerConfig{
		Level: DebugLevel,
		Out:   os.Stdout,
	}
)

type LoggerConfig struct {
	Level int32 `json:"level"`
	Out   io.Writer
}

type Logger struct {
	config LoggerConfig
}

func (l *Logger) Debug(msg string) { l.write(colorString(BLUE, "DEBUG: "+msg)) }
func (l *Logger) Info(msg string)  { l.write(colorString(GREEN, "INFO: "+msg)) }
func (l *Logger) Warn(msg string)  { l.write(colorString(YELLOW, "WARN: "+msg)) }
func (l *Logger) Error(msg string) { l.write(colorString(RED, "ERROR: "+msg)) }
func (l *Logger) Print(msg string) { l.write(msg) }

func (l *Logger) Debugf(format string, args ...interface{}) {
	l.write(colorStringWithFormat(BLUE, "DEBUG: "+format, args...))
}

func (l *Logger) Infof(format string, args ...interface{}) {
	l.write(colorStringWithFormat(GREEN, "INFO: "+format, args...))
}

func (l *Logger) Warnf(format string, args ...interface{}) {
	l.write(colorStringWithFormat(YELLOW, "WARN: "+format, args...))
}

func (l *Logger) Errorf(format string, args ...interface{}) {
	l.write(colorStringWithFormat(RED, "ERROR: "+format, args...))
}

func (l *Logger) Printf(format string, args ...interface{}) {
	l.write(fmt.Sprintf(format, args...))
}

func (l *Logger) write(msg string) {
	timeColored := colorString(GRAY, time.Now().Format(time.DateTime))
	if _, err := l.config.Out.Write([]byte(fmt.Sprintf("%s %s\n", timeColored, msg))); err != nil {
		fmt.Println(newLogError(err))
	}
}

func NewLogger(config LoggerConfig) LoggerI {
	if config.Out == nil {
		config.Out = os.Stdout
	}
	return &Logger{
		config: config,
	}
}

func NewDefaultLogger() LoggerI {
	return NewLogger(DefaultLoggerConfig)
}

func colorStringWithFormat(c string, format string, args ...interface{}) string {
	return c + fmt.Sprintf(format, args...) + Reset
}

func colorString(c, msg string) string {
	return c + msg + Reset
}

package lib

import (
	"bytes"
	"strings"
	"testing"
)

func TestLogger(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		level  int32
		color  string
	}{
		{
			name:   "info",
			prefix: "INFO:",
			level:  InfoLevel,
			color:  GREEN,
		},
		{
			name:   "debug",
			prefix: "DEBUG:",
			level:  DebugLevel,
			color:  BLUE,
		},
		{
			name:   "warn",
			prefix: "WARN:",
			level:  WarnLevel,
			color:  YELLOW,
		},
		{
			name:   "error",
			prefix: "ERROR:",
			level:  ErrorLevel,
			color:  RED,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			buf := bytes.NewBuffer(nil)
			logger := NewLogger(LoggerConfig{
				Level: DebugLevel,
				Out:   buf,
			})
			expectedString := test.color + test.prefix + " arg1 arg2" + Reset
			switch test.level {
			case InfoLevel:
				logger.Info("arg1 arg2")
			case DebugLevel:
				logger.Debug("arg1 arg2")
			case ErrorLevel:
				logger.Error("arg1 arg2")
			case WarnLevel:
				logger.Warn("arg1 arg2")
			}
			got := buf.String()
			if !strings.Contains(got, expectedString) {
				t.Fatalf("wanted %s to contain %s", got, expectedString)
			}
		})
	}
}

func TestLoggerFormat(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		level  int32
		color  string
	}{
		{
			name:   "info",
			prefix: "INFO:",
			level:  InfoLevel,
			color:  GREEN,
		},
		{
			name:   "debug",
			prefix: "DEBUG:",
			level:  DebugLevel,
			color:  BLUE,
		},
		{
			name:   "warn",
			prefix: "WARN:",
			level:  WarnLevel,
			color:  YELLOW,
		},
		{
			name:   "error",
			prefix: "ERROR:",
			level:  ErrorLevel,
			color:  RED,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			buf := bytes.NewBuffer(nil)
			logger := NewLogger(LoggerConfig{
				Level: DebugLevel,
				Out:   buf,
			})
			expectedString := test.color + test.prefix + " arg1 arg2" + Reset
			switch test.level {
			case InfoLevel:
				logger.Infof("%s %s", "arg1", "arg2")
			case DebugLevel:
				logger.Debugf("%s %s", "arg1", "arg2")
			case ErrorLevel:
				logger.Errorf("%s %s", "arg1", "arg2")
			case WarnLevel:
				logger.Warnf("%s %s", "arg1", "arg2")
			}
			got := buf.String()
			if !strings.Contains(got, expectedString) {
				t.Fatalf("wanted %s to contain %s", got, expectedString)
			}
		})
	}
}

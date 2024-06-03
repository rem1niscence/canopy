package lib

import (
	"errors"
	"fmt"
	lumberjack "gopkg.in/natefinch/lumberjack.v2"
	"io"
	"os"
	"path/filepath"
	"time"
)

const (
	LogDirectory = "logs"
	LogFileName  = "log"
)

type LoggerI interface {
	Debug(msg string)
	Info(msg string)
	Warn(msg string)
	Error(msg string)
	Fatal(msg string)
	Print(msg string)
	Debugf(format string, args ...interface{})
	Infof(format string, args ...interface{})
	Warnf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
	Fatalf(format string, args ...interface{})
	Printf(format string, args ...interface{})
}

const (
	DebugLevel int32 = -4
	InfoLevel  int32 = 0
	WarnLevel  int32 = 4
	ErrorLevel int32 = 8

	Reset  = "\033[0m"
	RED    = "\033[31m"
	GREEN  = "\033[32m"
	YELLOW = "\033[33m"
	BLUE   = "\033[34m"
	GRAY   = "\033[37m"
)

var (
	_ LoggerI = &Logger{}
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
func (l *Logger) Fatal(msg string) {
	l.Error(msg)
	os.Exit(1)
}

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
func (l *Logger) Fatalf(format string, args ...interface{}) {
	l.Errorf(format, args)
	os.Exit(1)
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

func NewLogger(config LoggerConfig, dataDirPath ...string) LoggerI {
	if config.Out == nil {
		if dataDirPath == nil || dataDirPath[0] == "" {
			dataDirPath = make([]string, 1)
			dataDirPath[0] = DefaultDataDirPath()
		}
		logPath := filepath.Join(dataDirPath[0], LogDirectory, LogFileName)
		if _, err := os.Stat(logPath); errors.Is(err, os.ErrNotExist) {
			if err = os.MkdirAll(filepath.Join(dataDirPath[0], LogDirectory), os.ModePerm); err != nil {
				panic(err)
			}
		}
		logFile := &lumberjack.Logger{
			Filename:   logPath,
			MaxSize:    1, // megabyte
			MaxBackups: 1500,
			MaxAge:     14, // days
			Compress:   true,
		}
		config.Out = io.MultiWriter(os.Stdout, logFile)
	}
	return &Logger{
		config: config,
	}
}

func NewDefaultLogger() LoggerI {
	return NewLogger(LoggerConfig{
		Level: DebugLevel,
		Out:   os.Stdout,
	})
}

func colorStringWithFormat(c string, format string, args ...interface{}) string {
	return c + fmt.Sprintf(format, args...) + Reset
}
func colorString(c, msg string) string { return c + msg + Reset }

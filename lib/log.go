package lib

import (
	"errors"
	"fmt"
	"github.com/fatih/color"
	"gopkg.in/natefinch/lumberjack.v2"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	LogDirectory = "logs"
	LogFileName  = "log"
)

func init() {
	color.NoColor = false
}

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

	Reset = iota
	RED
	GREEN
	YELLOW
	BLUE
	GRAY
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

func colorStringWithFormat(c int, format string, args ...interface{}) string {
	return colorString(c, fmt.Sprintf(format, args...))
}
func colorString(c int, msg string) (res string) {
	arr := strings.Split(msg, "\n")
	l := len(arr)
	for i, part := range arr {
		res += cString(c, part)
		if i != l-1 {
			res += "\n"
		}
	}
	return
}

func cString(c int, msg string) string {
	switch c {
	case BLUE:
		return color.BlueString(msg)
	case RED:
		return color.RedString(msg)
	case YELLOW:
		return color.YellowString(msg)
	case GREEN:
		return color.GreenString(msg)
	case GRAY:
		return color.HiBlackString(msg)
	default:
		return color.WhiteString(msg)
	}
}

package logger

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"github.com/mattn/go-colorable"
	log "github.com/sirupsen/logrus"
)

var LogEntry *log.Entry

func LogCloser() error {
	return closer()
}

// Deprecated: logger.LogEntry
func GetLoggerEntry() *log.Entry {
	return LogEntry
}

type Config struct {
	Level  string
	Path   string
	Rotate string
}

var closer = func() error {
	return nil
}

func init() {
	fmt.Println("init log 。。。。")
	logCfg := Config{Level: "info", Path: "stdout"}
	newLogger := log.New()
	// 如果是终端输出，使用带颜色的文本格式；否则用JSON格式
	if logCfg.Path == "" || logCfg.Path == "stdout" || logCfg.Path == "stderr" {
		newLogger.SetFormatter(&log.TextFormatter{
			ForceColors:     true,
			FullTimestamp:   true,
			TimestampFormat: "2006-01-02 15:04:05",
		})
	} else {
		newLogger.SetFormatter(&log.JSONFormatter{
			TimestampFormat: "2006-01-02 15:04:05",
		})
	}

	// Log as JSON instead of the default ASCII formatter.
	// newLogger.SetFormatter(&log.JSONFormatter{})
	// Only log the warning severity or above.

	switch strings.ToLower(logCfg.Level) {
	case "debug":
		newLogger.SetLevel(log.DebugLevel)
	case "info":
		newLogger.SetLevel(log.InfoLevel)
	case "warn":
		newLogger.SetLevel(log.WarnLevel)
	case "error":
		newLogger.SetLevel(log.ErrorLevel)
	default:
		panic(errors.New("LogLevel " + logCfg.Level + " not found"))
	}

	// Output to stdout instead of the default stderr
	// Can be any io.Writer, see below for File example
	switch strings.ToLower(logCfg.Path) {
	case "":
		fallthrough
	case "stdout":
		newLogger.SetOutput(colorable.NewColorableStdout())
	case "stderr":
		newLogger.SetOutput(colorable.NewColorableStderr())
	default:
		// 检查文件路径
		fileInfo, err := os.Stat(logCfg.Path)
		if err == nil && fileInfo.IsDir() {
			// 不能是 目录
			// https://haicoder.net/golang/golang-bufio.html
			// https://golangnote.com/topic/92.html
			panic("path is dir")
		}
		// 存在 日志分割配置
		logs, err := rotatelogs.New(
			logCfg.Path+"."+logCfg.Rotate,
			rotatelogs.WithLinkName(logCfg.Path),
			rotatelogs.WithMaxAge(24*time.Hour),
			rotatelogs.WithRotationTime(time.Hour),
		)
		if err != nil {
			panic(fmt.Sprintf("rotate log init error: %v", err))
		}
		newLogger.SetOutput(logs)
		closer = logs.Close
	}
	LogEntry = log.NewEntry(newLogger)
}

func PrintError(err error) {
	LogEntry.Error(err)
}

// 直接暴露 info 函数，简化调用
func Info(args ...interface{}) {
	LogEntry.Info(args...)
}

// 直接暴露 error 函数，简化调用
func Errorf(format string, args ...interface{}) {
	LogEntry.Errorf(format, args...)
}

// 直接暴露 error 函数，简化调用
func Fatalf(format string, args ...interface{}) {
	LogEntry.Fatalf(format, args...)
}

// 直接暴露 error 函数，简化调用
func Printf(format string, args ...interface{}) {
	LogEntry.Printf(format, args...)
}

// 直接暴露 error 函数，简化调用
func Warn(args ...interface{}) {
	LogEntry.Warn(args...)
}

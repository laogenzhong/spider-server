package logger

import (
	"errors"
	"fmt"
	"io"
	stdlog "log"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
	Level        string
	Path         string
	ErrorPath    string
	Rotate       string
	MaxAge       time.Duration
	RotationTime time.Duration
	MaxSizeMB    int
	Format       string
}

var closer = func() error {
	return nil
}

func init() {
	Configure(Config{Level: "info", Path: "stdout", Rotate: "%Y%m%d%H", MaxAge: 24 * time.Hour, RotationTime: time.Hour, MaxSizeMB: 0})
}

func Configure(logCfg Config) {
	_ = closer()
	closer = func() error {
		return nil
	}

	if logCfg.Level == "" {
		logCfg.Level = "info"
	}
	if logCfg.Path == "" {
		logCfg.Path = "stdout"
	}
	if logCfg.Rotate == "" {
		logCfg.Rotate = "%Y%m%d%H"
	}
	if logCfg.MaxAge <= 0 {
		logCfg.MaxAge = 24 * time.Hour
	}
	if logCfg.RotationTime <= 0 {
		logCfg.RotationTime = time.Hour
	}
	if logCfg.Format == "" {
		logCfg.Format = "plain"
	}

	newLogger := log.New()
	formatter := formatterForConfig(logCfg)
	// 终端输出保留带颜色文本；文件输出支持 plain/json 两种格式。
	if logCfg.Path == "" || logCfg.Path == "stdout" || logCfg.Path == "stderr" {
		newLogger.SetFormatter(formatter)
	} else {
		newLogger.SetFormatter(formatter)
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
		stdlog.SetOutput(colorable.NewColorableStdout())
	case "stderr":
		newLogger.SetOutput(colorable.NewColorableStderr())
		stdlog.SetOutput(colorable.NewColorableStderr())
	default:
		// 检查文件路径
		fileInfo, err := os.Stat(logCfg.Path)
		if err == nil && fileInfo.IsDir() {
			// 不能是 目录
			// https://haicoder.net/golang/golang-bufio.html
			// https://golangnote.com/topic/92.html
			panic("path is dir")
		}
		logs, rotateErr := newRotateWriter(logCfg.Path, logCfg)
		if rotateErr != nil {
			panic(fmt.Sprintf("rotate log init error: %v", rotateErr))
		}
		newLogger.SetOutput(logs)
		stdlog.SetOutput(logs)
		closer = logs.Close

		if strings.TrimSpace(logCfg.ErrorPath) != "" {
			errorLogs, rotateErr := newRotateWriter(logCfg.ErrorPath, logCfg)
			if rotateErr != nil {
				_ = logs.Close()
				panic(fmt.Sprintf("error rotate log init error: %v", rotateErr))
			}
			newLogger.AddHook(&errorMirrorHook{
				writer:    errorLogs,
				formatter: formatter,
			})
			closer = closeAll(logs.Close, errorLogs.Close)
		}
	}
	stdlog.SetFlags(stdlog.LstdFlags | stdlog.Lshortfile)
	LogEntry = log.NewEntry(newLogger)
}

type rotateWriter interface {
	Write([]byte) (int, error)
	Close() error
}

func formatterForConfig(logCfg Config) log.Formatter {
	if logCfg.Path == "" || logCfg.Path == "stdout" || logCfg.Path == "stderr" {
		return &log.TextFormatter{
			ForceColors:     true,
			FullTimestamp:   true,
			TimestampFormat: "2006-01-02 15:04:05",
		}
	}

	switch strings.ToLower(strings.TrimSpace(logCfg.Format)) {
	case "json":
		return &log.JSONFormatter{
			TimestampFormat: "2006-01-02 15:04:05",
		}
	case "", "plain", "text":
		return plainFormatter{
			TimestampFormat: "2006-01-02 15:04:05",
		}
	default:
		panic(errors.New("LogFormat " + logCfg.Format + " not found"))
	}
}

func newRotateWriter(path string, logCfg Config) (rotateWriter, error) {
	if logCfg.MaxSizeMB > 0 {
		return newDailySizeRotateWriter(path, logCfg.MaxSizeMB, logCfg.MaxAge)
	}
	return rotatelogs.New(
		path+"."+logCfg.Rotate,
		rotatelogs.WithLinkName(path),
		rotatelogs.WithMaxAge(logCfg.MaxAge),
		rotatelogs.WithRotationTime(logCfg.RotationTime),
	)
}

func closeAll(closers ...func() error) func() error {
	return func() error {
		var firstErr error
		for _, closeFn := range closers {
			if closeFn == nil {
				continue
			}
			if err := closeFn(); err != nil && firstErr == nil {
				firstErr = err
			}
		}
		return firstErr
	}
}

type errorMirrorHook struct {
	mu        sync.Mutex
	writer    io.Writer
	formatter log.Formatter
}

func (h *errorMirrorHook) Levels() []log.Level {
	return []log.Level{log.PanicLevel, log.FatalLevel, log.ErrorLevel}
}

func (h *errorMirrorHook) Fire(entry *log.Entry) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	data, err := h.formatter.Format(entry)
	if err != nil {
		return err
	}
	_, err = h.writer.Write(data)
	return err
}

type plainFormatter struct {
	TimestampFormat string
}

func (f plainFormatter) Format(entry *log.Entry) ([]byte, error) {
	timestampFormat := f.TimestampFormat
	if timestampFormat == "" {
		timestampFormat = time.RFC3339
	}

	message := strings.TrimRight(entry.Message, "\r\n")
	return []byte(entry.Time.Format(timestampFormat) + " " + message + "\n"), nil
}

type dailySizeRotateWriter struct {
	mu        sync.Mutex
	basePath  string
	maxSizeMB int
	maxAge    time.Duration
	date      string
	file      *os.File
	size      int64
}

func newDailySizeRotateWriter(basePath string, maxSizeMB int, maxAge time.Duration) (*dailySizeRotateWriter, error) {
	if err := os.MkdirAll(filepath.Dir(basePath), 0755); err != nil {
		return nil, err
	}
	writer := &dailySizeRotateWriter{
		basePath:  basePath,
		maxSizeMB: maxSizeMB,
		maxAge:    maxAge,
	}
	if err := writer.rotateIfNeeded(time.Now()); err != nil {
		return nil, err
	}
	return writer, nil
}

func (w *dailySizeRotateWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.rotateIfNeeded(time.Now()); err != nil {
		return 0, err
	}
	if w.size > 0 && w.maxSizeBytes() > 0 && w.size+int64(len(p)) > w.maxSizeBytes() {
		if err := w.rotateBySize(); err != nil {
			return 0, err
		}
	}
	n, err := w.file.Write(p)
	w.size += int64(n)
	return n, err
}

func (w *dailySizeRotateWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file == nil {
		return nil
	}
	return w.file.Close()
}

func (w *dailySizeRotateWriter) rotateIfNeeded(now time.Time) error {
	date := now.Format("20060102")
	if w.file != nil && w.date == date {
		return nil
	}
	if w.file != nil {
		_ = w.file.Close()
	}

	filename := dailyLogPath(w.basePath, date)
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return err
	}
	w.date = date
	w.file = file
	w.size = info.Size()
	updateCurrentLogLink(w.basePath, filename)
	w.cleanupExpiredDailyLogs(now)
	return nil
}

func (w *dailySizeRotateWriter) rotateBySize() error {
	if w.file != nil {
		_ = w.file.Close()
	}

	currentPath := dailyLogPath(w.basePath, w.date)
	archivePath, err := nextArchivePath(w.basePath, w.date)
	if err != nil {
		return err
	}
	if err := os.Rename(currentPath, archivePath); err != nil && !os.IsNotExist(err) {
		return err
	}

	file, err := os.OpenFile(currentPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	w.file = file
	w.size = 0
	updateCurrentLogLink(w.basePath, currentPath)
	return nil
}

func (w *dailySizeRotateWriter) maxSizeBytes() int64 {
	return int64(w.maxSizeMB) * 1024 * 1024
}

func dailyLogPath(basePath string, date string) string {
	ext := filepath.Ext(basePath)
	prefix := strings.TrimSuffix(basePath, ext)
	if ext == "" {
		return prefix + "." + date
	}
	return prefix + "." + date + ext
}

func updateCurrentLogLink(linkPath string, targetPath string) {
	_ = os.Remove(linkPath)
	_ = os.Symlink(filepath.Base(targetPath), linkPath)
}

func nextArchivePath(basePath string, date string) (string, error) {
	ext := filepath.Ext(basePath)
	prefix := strings.TrimSuffix(basePath, ext)
	for index := 1; index <= 9999; index++ {
		name := fmt.Sprintf("%s.%s.%03d%s", prefix, date, index, ext)
		if _, err := os.Stat(name); os.IsNotExist(err) {
			return name, nil
		}
	}
	return "", fmt.Errorf("too many rotated log files for %s", date)
}

func (w *dailySizeRotateWriter) cleanupExpiredDailyLogs(now time.Time) {
	if w.maxAge <= 0 {
		return
	}
	cutoff := now.Add(-w.maxAge)
	ext := filepath.Ext(w.basePath)
	prefix := strings.TrimSuffix(filepath.Base(w.basePath), ext)
	pattern := filepath.Join(filepath.Dir(w.basePath), prefix+".*"+ext+"*")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return
	}
	for _, match := range matches {
		info, err := os.Stat(match)
		if err != nil || info.IsDir() || info.ModTime().After(cutoff) {
			continue
		}
		_ = os.Remove(match)
	}
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

func Fatal(args ...interface{}) {
	LogEntry.Fatal(args...)
}

// 直接暴露 error 函数，简化调用
func Printf(format string, args ...interface{}) {
	LogEntry.Printf(format, args...)
}

func Println(args ...interface{}) {
	LogEntry.Println(args...)
}

func Writer() io.Writer {
	return logEntryWriter{}
}

type logEntryWriter struct{}

func (logEntryWriter) Write(p []byte) (int, error) {
	message := strings.TrimSpace(string(p))
	if message != "" {
		LogEntry.Print(message)
	}
	return len(p), nil
}

// 直接暴露 error 函数，简化调用
func Warn(args ...interface{}) {
	LogEntry.Warn(args...)
}

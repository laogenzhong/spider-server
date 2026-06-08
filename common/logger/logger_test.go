package logger

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
)

func TestDailySizeRotateWriterArchivesCurrentDayBySize(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "spider-server.log")
	writer, err := newDailySizeRotateWriter(basePath, 1, 7*24*time.Hour)
	if err != nil {
		t.Fatalf("newDailySizeRotateWriter failed: %v", err)
	}
	defer writer.Close()

	chunk := bytes.Repeat([]byte("x"), 1024*1024+128)
	if _, err := writer.Write(chunk); err != nil {
		t.Fatalf("first write failed: %v", err)
	}
	if _, err := writer.Write([]byte("next log line\n")); err != nil {
		t.Fatalf("second write failed: %v", err)
	}

	date := time.Now().Format("20060102")
	currentPath := dailyLogPath(basePath, date)
	archivePath := filepath.Join(dir, "spider-server."+date+".001.log")
	if _, err := os.Stat(currentPath); err != nil {
		t.Fatalf("current daily log missing: %v", err)
	}
	if _, err := os.Stat(archivePath); err != nil {
		t.Fatalf("size archive log missing: %v", err)
	}
	if target, err := os.Readlink(basePath); err == nil && target != filepath.Base(currentPath) {
		t.Fatalf("current log link target = %q, want %q", target, filepath.Base(currentPath))
	}
}

func TestPlainFormatterFormatsTimeAndMessage(t *testing.T) {
	formatter := plainFormatter{TimestampFormat: "2006-01-02 15:04:05"}
	entry := &log.Entry{
		Time:    time.Date(2026, 6, 9, 10, 11, 12, 0, time.Local),
		Level:   log.InfoLevel,
		Message: "hello world",
		Data: log.Fields{
			"uid":   uint64(7),
			"order": "abc",
		},
	}

	data, err := formatter.Format(entry)
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	got := string(data)
	want := "2026-06-09 10:11:12 hello world\n"
	if got != want {
		t.Fatalf("formatted log = %q, want %q", got, want)
	}
}

func TestConfigureFileLoggerCanUseJSONFormat(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "spider-server.log")
	Configure(Config{
		Level:        "info",
		Path:         basePath,
		Format:       "json",
		Rotate:       "%Y%m%d",
		MaxAge:       24 * time.Hour,
		RotationTime: 24 * time.Hour,
		MaxSizeMB:    1,
	})
	defer Configure(Config{Level: "info", Path: "stdout"})

	Info("json format works")
	if err := LogCloser(); err != nil {
		t.Fatalf("LogCloser failed: %v", err)
	}

	currentPath := dailyLogPath(basePath, time.Now().Format("20060102"))
	data, err := os.ReadFile(currentPath)
	if err != nil {
		t.Fatalf("read json log failed: %v", err)
	}
	if !strings.Contains(string(data), `"msg":"json format works"`) {
		t.Fatalf("json log missing message: %s", data)
	}
}

func TestConfigureMirrorsErrorLogsToErrorPath(t *testing.T) {
	dir := t.TempDir()
	infoPath := filepath.Join(dir, "info", "spider-server.log")
	errorPath := filepath.Join(dir, "error", "spider-server.log")
	Configure(Config{
		Level:        "info",
		Path:         infoPath,
		ErrorPath:    errorPath,
		Format:       "plain",
		Rotate:       "%Y%m%d",
		MaxAge:       24 * time.Hour,
		RotationTime: 24 * time.Hour,
		MaxSizeMB:    1,
	})
	defer Configure(Config{Level: "info", Path: "stdout"})

	Info("normal info line")
	Errorf("error line %d", 1)
	if err := LogCloser(); err != nil {
		t.Fatalf("LogCloser failed: %v", err)
	}

	date := time.Now().Format("20060102")
	infoData, err := os.ReadFile(dailyLogPath(infoPath, date))
	if err != nil {
		t.Fatalf("read info log failed: %v", err)
	}
	errorData, err := os.ReadFile(dailyLogPath(errorPath, date))
	if err != nil {
		t.Fatalf("read error log failed: %v", err)
	}

	infoText := string(infoData)
	if !strings.Contains(infoText, "normal info line") || !strings.Contains(infoText, "error line 1") {
		t.Fatalf("info log should contain both info and error lines: %s", infoText)
	}
	errorText := string(errorData)
	if strings.Contains(errorText, "normal info line") || !strings.Contains(errorText, "error line 1") {
		t.Fatalf("error log should contain only mirrored error lines: %s", errorText)
	}
}

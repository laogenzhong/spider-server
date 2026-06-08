package logger

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
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

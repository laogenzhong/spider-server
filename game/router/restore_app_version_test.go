package router

import (
	"strings"
	"testing"
)

func TestNormalizeRestoreAppVersion(t *testing.T) {
	if got := normalizeRestoreAppVersion("  1.6.0  "); got != "1.6.0" {
		t.Fatalf("normalizeRestoreAppVersion() = %q, want %q", got, "1.6.0")
	}

	got := normalizeRestoreAppVersion(strings.Repeat("版", 40))
	if runeCount := len([]rune(got)); runeCount != 32 {
		t.Fatalf("normalized rune count = %d, want 32", runeCount)
	}
}

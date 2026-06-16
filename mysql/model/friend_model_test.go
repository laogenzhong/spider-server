package mysqlmodel

import (
	"strings"
	"testing"
)

func TestNormalizeFriendNickname(t *testing.T) {
	nickname := normalizeFriendNickname("  Apple 用户  ")
	if nickname != "Apple 用户" {
		t.Fatalf("nickname = %q, want %q", nickname, "Apple 用户")
	}
}

func TestNormalizeFriendNicknameTruncatesByRune(t *testing.T) {
	nickname := normalizeFriendNickname(strings.Repeat("练", 70))
	if len([]rune(nickname)) != 64 {
		t.Fatalf("rune length = %d, want 64", len([]rune(nickname)))
	}
}

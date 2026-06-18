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

func TestDefaultFriendProfileUsesFirstCustomAvatar(t *testing.T) {
	profile := defaultFriendProfile(7)
	if profile.AvatarSymbol != defaultFriendAvatarSymbol {
		t.Fatalf("avatar symbol = %q, want %q", profile.AvatarSymbol, defaultFriendAvatarSymbol)
	}
	if profile.AvatarSymbol != "profile_avatar_1" {
		t.Fatalf("avatar symbol = %q, want profile_avatar_1", profile.AvatarSymbol)
	}
}

func TestNormalizeFriendAvatarSymbol(t *testing.T) {
	tests := map[string]string{
		"":                  "profile_avatar_1",
		"person.fill":       "profile_avatar_1",
		"profile_avatar_0":  "profile_avatar_1",
		"profile_avatar_21": "profile_avatar_1",
		"profile_avatar_7":  "profile_avatar_7",
		"  12  ":            "profile_avatar_12",
	}

	for input, want := range tests {
		if got := normalizeFriendAvatarSymbol(input); got != want {
			t.Fatalf("normalizeFriendAvatarSymbol(%q) = %q, want %q", input, got, want)
		}
	}
}

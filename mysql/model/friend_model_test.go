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

func TestParseDefaultFriendUserIDIgnoresCase(t *testing.T) {
	uid, ok := parseDefaultFriendUserID(" sp000008 ")
	if !ok {
		t.Fatalf("ok = false, want true")
	}
	if uid != 8 {
		t.Fatalf("uid = %d, want 8", uid)
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

func TestParseFriendSharedPlanPreservesExerciseSets(t *testing.T) {
	plan, err := ParseFriendSharedPlan(`{"title":"上肢训练","source_plan_id":"plan-1","exercises":[{"exercise_id":"bench_press","name_snapshot":"杠铃卧推","custom_introduction":"保持肩胛骨稳定","set_count":1,"sets":[{"weight_text":"80","reps_text":"8"}]}]}`)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Title != "上肢训练" || plan.SourcePlanID != "plan-1" || len(plan.Exercises) != 1 || len(plan.Exercises[0].Sets) != 1 || plan.Exercises[0].CustomIntroduction != "保持肩胛骨稳定" {
		t.Fatalf("plan = %#v", plan)
	}
}

func TestFriendTrainingScoreTitleUsesCustomName(t *testing.T) {
	title := friendTrainingScoreTitle(FriendActionTrainingSessionRecord{
		Exercises: []FriendActionExerciseSummaryRecord{{CustomName: "自定义推举"}, {NameSnapshot: "划船"}},
	})
	if title != "自定义推举 等 2 个动作" {
		t.Fatalf("title = %q", title)
	}
}

func TestFriendPlanShareDeleteReason(t *testing.T) {
	if reason, ok := friendPlanShareDeleteReason(FriendPlanShareDispositionUsed); !ok || reason != "used" {
		t.Fatalf("used reason = %q, ok = %v", reason, ok)
	}
	if reason, ok := friendPlanShareDeleteReason(FriendPlanShareDispositionIgnored); !ok || reason != "ignored" {
		t.Fatalf("ignored reason = %q, ok = %v", reason, ok)
	}
	if _, ok := friendPlanShareDeleteReason(0); ok {
		t.Fatal("unknown disposition unexpectedly accepted")
	}
}

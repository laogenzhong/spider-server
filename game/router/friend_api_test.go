package router

import (
	"strings"
	"testing"

	gamecode "spider-server/game/code"
	pb "spider-server/gen/spider/api"
)

func TestValidateFriendTrainingSnapshotAcceptsBoundActionTraining(t *testing.T) {
	snapshot := validFriendTrainingSnapshotForTest()
	if code := validateFriendTrainingSnapshot(snapshot); code != 0 {
		t.Fatalf("validation code = %d, want 0", code)
	}

	restored := friendTrainingDaysToPB(friendTrainingDaysFromPB(snapshot.GetRecentTrainingDays()))
	if len(restored) != 1 || len(restored[0].GetActionTrainingSessions()) != 1 {
		t.Fatalf("restored days = %#v", restored)
	}
	training := restored[0].GetActionTrainingSessions()[0]
	if training.GetBoundWorkout().GetWorkoutType() != "manual_workout_type_strength" || !training.GetBoundWorkout().GetHasDistance() || training.GetBoundWorkout().GetDistanceMeters() != 1250 {
		t.Fatalf("restored bound workout = %#v", training.GetBoundWorkout())
	}
	if training.GetExercises()[0].GetCustomIntroduction() != "保持躯干稳定" {
		t.Fatalf("restored custom introduction = %q", training.GetExercises()[0].GetCustomIntroduction())
	}
}

func TestValidateFriendTrainingSnapshotRejectsUnboundedDetail(t *testing.T) {
	snapshot := validFriendTrainingSnapshotForTest()
	for day := 1; day < friendTrainingSnapshotActionDetailDays; day++ {
		copyDay := *snapshot.RecentTrainingDays[0]
		copyDay.RecordDate = "2026-07-1" + string(rune('0'+day))
		copyDay.ActionTrainingSessions = nil
		snapshot.RecentTrainingDays = append(snapshot.RecentTrainingDays, &copyDay)
	}
	copyDay := *snapshot.RecentTrainingDays[0]
	copyDay.RecordDate = "2026-07-20"
	snapshot.RecentTrainingDays = append(snapshot.RecentTrainingDays, &copyDay)

	if code := validateFriendTrainingSnapshot(snapshot); code != gamecode.FriendTrainingSnapshotInvalid {
		t.Fatalf("validation code = %d, want %d", code, gamecode.FriendTrainingSnapshotInvalid)
	}
}

func TestValidateFriendTrainingSnapshotRejectsTooLargePayload(t *testing.T) {
	snapshot := validFriendTrainingSnapshotForTest()
	snapshot.RecentTrainingDays[0].ActionTrainingSessions[0].Exercises[0].NameSnapshot = strings.Repeat("x", friendTrainingSnapshotMaxBytes)

	if code := validateFriendTrainingSnapshot(snapshot); code != gamecode.FriendTrainingSnapshotTooLarge {
		t.Fatalf("validation code = %d, want %d", code, gamecode.FriendTrainingSnapshotTooLarge)
	}
}

func TestValidateFriendSharedPlanAndJSONRoundTrip(t *testing.T) {
	plan := validFriendSharedPlanForTest()
	if code := validateFriendSharedPlan("client-share-1", plan); code != 0 {
		t.Fatalf("validation code = %d, want 0", code)
	}
	restored := friendSharedPlanToPB(friendSharedPlanFromPB(plan))
	if restored.GetTitle() != plan.GetTitle() || restored.GetSourcePlanId() != plan.GetSourcePlanId() || len(restored.GetExercises()) != 1 || len(restored.GetExercises()[0].GetSets()) != 2 {
		t.Fatalf("restored plan = %#v", restored)
	}
	if restored.GetExercises()[0].GetCustomIntroduction() != plan.GetExercises()[0].GetCustomIntroduction() {
		t.Fatalf("custom introduction = %q, want %q", restored.GetExercises()[0].GetCustomIntroduction(), plan.GetExercises()[0].GetCustomIntroduction())
	}
}

func TestValidateFriendSharedPlanRejectsTooLargePayload(t *testing.T) {
	plan := validFriendSharedPlanForTest()
	plan.Exercises[0].Note = strings.Repeat("x", friendSharedPlanMaxBytes)
	if code := validateFriendSharedPlan("client-share-1", plan); code != gamecode.FriendPlanShareTooLarge {
		t.Fatalf("validation code = %d, want %d", code, gamecode.FriendPlanShareTooLarge)
	}
}

func validFriendSharedPlanForTest() *pb.FriendSharedPlan {
	return &pb.FriendSharedPlan{
		Title:        "上肢训练",
		SourcePlanId: "5f1efc3f-8d1d-4ce7-989d-bf78179eb1ab",
		Exercises: []*pb.FriendSharedPlanExercise{{
			ExerciseId:         "custom-friend-bench",
			NameKey:            "好友卧推",
			NameSnapshot:       "好友卧推",
			CategoryKey:        "exercise_category_chest",
			TypeKey:            "exercise_type_barbell",
			DisplayTypeKey:     "exercise_type_barbell",
			CustomName:         "好友卧推",
			CustomIntroduction: "保持肩胛骨稳定",
			SetCount:           2,
			WeightUnit:         "kg",
			Sets: []*pb.FriendSharedPlanSet{
				{WeightText: "80", RepsText: "8"},
				{WeightText: "80", RepsText: "8"},
			},
		}},
	}
}

func validFriendTrainingSnapshotForTest() *pb.MyTrainingPublicSnapshot {
	distance := 1250.0
	return &pb.MyTrainingPublicSnapshot{
		Visible:   true,
		SparkDays: 4,
		UpdatedAt: 1784505600000,
		RecentTrainingDays: []*pb.FriendTrainingDaySummary{{
			RecordDate: "2026-07-20",
			Calories:   "320",
			Tags: []*pb.FriendTrainingTagStat{{
				Name:     "manual_workout_type_strength",
				Calories: "320",
			}},
			ActionTrainingSessions: []*pb.FriendActionTrainingSession{{
				SessionId: "1784505600000-0",
				StartAt:   1784505600000,
				EndAt:     1784509200000,
				Kind:      pb.FriendActionTrainingKind_FRIEND_ACTION_TRAINING_KIND_STRENGTH,
				Exercises: []*pb.FriendActionExerciseSummary{{
					ExerciseId:         "bench_press",
					NameKey:            "exercise_bench_press",
					NameSnapshot:       "杠铃卧推",
					CategoryKey:        "exercise_category_chest",
					TypeKey:            "exercise_type_barbell",
					CustomName:         "好友卧推",
					CustomIntroduction: "保持躯干稳定",
					Sets: []*pb.FriendActionSetSummary{{
						WeightX10:  800,
						WeightUnit: pb.FriendActionWeightUnit_FRIEND_ACTION_WEIGHT_UNIT_KG,
						Reps:       8,
					}},
				}},
				BoundWorkout: &pb.FriendBoundWorkoutSummary{
					WorkoutType:     "manual_workout_type_strength",
					StartAt:         1784505500000,
					EndAt:           1784509300000,
					DurationSeconds: 3800,
					EnergyKcal:      320,
					DistanceMeters:  distance,
					HasDistance:     true,
					Tags:            []string{"胸"},
				},
			}},
		}},
	}
}

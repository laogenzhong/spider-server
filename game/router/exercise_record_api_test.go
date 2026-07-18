package router

import (
	"fmt"
	"strings"
	"testing"

	appconfig "spider-server/common/config"
	pb "spider-server/gen/spider/api"
)

func TestConfigureWorkoutDataSyncLimits(t *testing.T) {
	defer ConfigureWorkoutDataSyncLimits(appconfig.Default().WorkoutDataSync)

	cfg := appconfig.Default().WorkoutDataSync
	cfg.SyncRPCMaxRequestBytes = 12345
	ConfigureWorkoutDataSyncLimits(cfg)
	if maxWorkoutDataSyncRequestBytes != 12345 {
		t.Fatalf("maxWorkoutDataSyncRequestBytes = %d, want 12345", maxWorkoutDataSyncRequestBytes)
	}
}

func TestValidWorkoutLibrarySnapshotLimits(t *testing.T) {
	tests := []struct {
		name    string
		library *pb.WorkoutLibrarySnapshot
		valid   bool
	}{
		{
			name:    "99 folders",
			library: &pb.WorkoutLibrarySnapshot{Folders: makeFolders(99)},
			valid:   true,
		},
		{
			name:    "100 folders",
			library: &pb.WorkoutLibrarySnapshot{Folders: makeFolders(100)},
			valid:   false,
		},
		{
			name: "99 plans",
			library: &pb.WorkoutLibrarySnapshot{Folders: []*pb.WorkoutPlanFolderSnapshot{{
				Id: "folder-id", Title: "Folder", Plans: makePlans(99),
			}}},
			valid: true,
		},
		{
			name: "100 plans",
			library: &pb.WorkoutLibrarySnapshot{Folders: []*pb.WorkoutPlanFolderSnapshot{{
				Id: "folder-id", Title: "Folder", Plans: makePlans(100),
			}}},
			valid: false,
		},
		{
			name:    "99 exercises",
			library: libraryWithExercises(makeExercises(99)),
			valid:   true,
		},
		{
			name:    "100 exercises",
			library: libraryWithExercises(makeExercises(100)),
			valid:   false,
		},
		{
			name:    "99 sets",
			library: libraryWithExercises([]*pb.WorkoutPlanExerciseSnapshot{{Id: "exercise-id", ExerciseId: "catalog-id", SetCount: 99, Sets: makeSets(99)}}),
			valid:   true,
		},
		{
			name:    "100 sets",
			library: libraryWithExercises([]*pb.WorkoutPlanExerciseSnapshot{{Id: "exercise-id", ExerciseId: "catalog-id", SetCount: 100, Sets: makeSets(100)}}),
			valid:   false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := validWorkoutLibrarySnapshot(test.library); got != test.valid {
				t.Fatalf("validWorkoutLibrarySnapshot() = %v, want %v", got, test.valid)
			}
		})
	}
}

func TestValidWorkoutDataSnapshotIncrementalEntities(t *testing.T) {
	base := &pb.WorkoutDataSnapshot{
		ClientSnapshotId: "snapshot-id",
		EntityId:         "plan-id",
		ChangedAt:        1,
		Kind:             pb.WorkoutDataSnapshotKind_WORKOUT_DATA_SNAPSHOT_KIND_PLAN,
		PlanEntity: &pb.WorkoutPlanEntitySnapshot{
			FolderId: "folder-id",
			Plan:     &pb.WorkoutPlanSnapshot{Id: "plan-id", Title: "Plan"},
		},
	}
	if !validWorkoutDataSnapshot(base) {
		t.Fatal("valid incremental plan snapshot was rejected")
	}

	deleted := protoCloneWorkoutSnapshot(base)
	deleted.Deleted = true
	deleted.PlanEntity = nil
	if !validWorkoutDataSnapshot(deleted) {
		t.Fatal("valid plan tombstone was rejected")
	}

	mismatched := protoCloneWorkoutSnapshot(base)
	mismatched.PlanEntity.Plan.Id = "different-plan-id"
	if validWorkoutDataSnapshot(mismatched) {
		t.Fatal("mismatched plan entity id was accepted")
	}

	oversizedTitle := protoCloneWorkoutSnapshot(base)
	oversizedTitle.PlanEntity.Plan.Title = strings.Repeat("x", maxWorkoutSnapshotTitleBytes+1)
	if validWorkoutDataSnapshot(oversizedTitle) {
		t.Fatal("oversized plan title was accepted")
	}
}

func TestValidWorkoutPlanFolderEntitySnapshotPlanIDs(t *testing.T) {
	folder := &pb.WorkoutPlanFolderEntitySnapshot{Id: "folder-id", Title: "Folder", PlanIds: make([]string, 99)}
	for index := range folder.PlanIds {
		folder.PlanIds[index] = fmt.Sprintf("plan-%d", index)
	}
	if !validWorkoutPlanFolderEntitySnapshot(folder) {
		t.Fatal("folder with 99 unique plan ids was rejected")
	}
	folder.PlanIds = append(folder.PlanIds, "plan-100")
	if validWorkoutPlanFolderEntitySnapshot(folder) {
		t.Fatal("folder with 100 plan ids was accepted")
	}
	folder.PlanIds = []string{"duplicate", "duplicate"}
	if validWorkoutPlanFolderEntitySnapshot(folder) {
		t.Fatal("folder with duplicate plan ids was accepted")
	}
}

func protoCloneWorkoutSnapshot(source *pb.WorkoutDataSnapshot) *pb.WorkoutDataSnapshot {
	clone := *source
	if source.PlanEntity != nil {
		entity := *source.PlanEntity
		clone.PlanEntity = &entity
		if source.PlanEntity.Plan != nil {
			plan := *source.PlanEntity.Plan
			clone.PlanEntity.Plan = &plan
		}
	}
	return &clone
}

func makeFolders(count int) []*pb.WorkoutPlanFolderSnapshot {
	result := make([]*pb.WorkoutPlanFolderSnapshot, count)
	for index := range result {
		result[index] = &pb.WorkoutPlanFolderSnapshot{Id: fmt.Sprintf("folder-%d", index), Title: "Folder"}
	}
	return result
}

func makePlans(count int) []*pb.WorkoutPlanSnapshot {
	result := make([]*pb.WorkoutPlanSnapshot, count)
	for index := range result {
		result[index] = &pb.WorkoutPlanSnapshot{Id: fmt.Sprintf("plan-%d", index), Title: "Plan"}
	}
	return result
}

func makeExercises(count int) []*pb.WorkoutPlanExerciseSnapshot {
	result := make([]*pb.WorkoutPlanExerciseSnapshot, count)
	for index := range result {
		result[index] = &pb.WorkoutPlanExerciseSnapshot{Id: fmt.Sprintf("exercise-%d", index), ExerciseId: fmt.Sprintf("catalog-%d", index), SetCount: 1}
	}
	return result
}

func makeSets(count int) []*pb.WorkoutPlanExerciseSetSnapshot {
	result := make([]*pb.WorkoutPlanExerciseSetSnapshot, count)
	for index := range result {
		result[index] = &pb.WorkoutPlanExerciseSetSnapshot{Id: fmt.Sprintf("set-%d", index)}
	}
	return result
}

func libraryWithExercises(exercises []*pb.WorkoutPlanExerciseSnapshot) *pb.WorkoutLibrarySnapshot {
	return &pb.WorkoutLibrarySnapshot{Folders: []*pb.WorkoutPlanFolderSnapshot{{
		Id: "folder-id", Title: "Folder", Plans: []*pb.WorkoutPlanSnapshot{{Id: "plan-id", Title: "Plan", Exercises: exercises}},
	}}}
}

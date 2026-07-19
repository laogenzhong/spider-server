package mysqlmodel

import (
	"testing"

	pb "spider-server/gen/spider/api"
)

func TestSupportedWorkoutDataSnapshotKinds(t *testing.T) {
	supportedKinds := []pb.WorkoutDataSnapshotKind{
		pb.WorkoutDataSnapshotKind_WORKOUT_DATA_SNAPSHOT_KIND_LIBRARY,
		pb.WorkoutDataSnapshotKind_WORKOUT_DATA_SNAPSHOT_KIND_LIBRARY_METADATA,
		pb.WorkoutDataSnapshotKind_WORKOUT_DATA_SNAPSHOT_KIND_PLAN_FOLDER,
		pb.WorkoutDataSnapshotKind_WORKOUT_DATA_SNAPSHOT_KIND_PLAN,
		pb.WorkoutDataSnapshotKind_WORKOUT_DATA_SNAPSHOT_KIND_TRAINING_SESSION,
	}
	for _, kind := range supportedKinds {
		if !isSupportedWorkoutDataSnapshotKind(kind) {
			t.Fatalf("kind %v should be supported", kind)
		}
	}
	if isSupportedWorkoutDataSnapshotKind(pb.WorkoutDataSnapshotKind_WORKOUT_DATA_SNAPSHOT_KIND_UNKNOWN) {
		t.Fatal("unknown kind should not be supported")
	}
}

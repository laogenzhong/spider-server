package mysqlmodel

import (
	"testing"

	appconfig "spider-server/common/config"
)

func TestConfigureWorkoutDataSnapshotLimits(t *testing.T) {
	defer ConfigureWorkoutDataSnapshotLimits(appconfig.Default().WorkoutDataSync)

	cfg := appconfig.Default().WorkoutDataSync
	cfg.SnapshotMaxPayloadBytes = 1234
	cfg.RestoreBatchTargetBytes = 2345
	cfg.RestoreBatchMaxSnapshots = 17
	ConfigureWorkoutDataSnapshotLimits(cfg)

	if maxWorkoutDataSnapshotPayloadBytes != 1234 || workoutDataSnapshotRestoreBatchBytes != 2345 || maxWorkoutDataSnapshotRestoreBatchRecords != 17 {
		t.Fatalf("configured limits = payload:%d batch_bytes:%d batch_records:%d", maxWorkoutDataSnapshotPayloadBytes, workoutDataSnapshotRestoreBatchBytes, maxWorkoutDataSnapshotRestoreBatchRecords)
	}
}

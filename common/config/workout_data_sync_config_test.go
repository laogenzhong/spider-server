package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadWorkoutDataSyncConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	data := []byte(`workout_data_sync:
  gateway_max_request_bytes: 11000000
  sync_rpc_max_request_bytes: 4200000
  snapshot_max_payload_bytes: 2100000
  restore_batch_max_snapshots: 321
  restore_batch_target_bytes: 1500000
`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	got := cfg.WorkoutDataSync
	if got.GatewayMaxRequestBytes != 11000000 || got.SyncRPCMaxRequestBytes != 4200000 || got.SnapshotMaxPayloadBytes != 2100000 || got.RestoreBatchMaxSnapshots != 321 || got.RestoreBatchTargetBytes != 1500000 {
		t.Fatalf("unexpected workout data sync config: %+v", got)
	}
}

func TestWorkoutDataSyncConfigFallsBackForNonPositiveValues(t *testing.T) {
	cfg := Config{WorkoutDataSync: WorkoutDataSyncConfig{}}
	cfg.Normalize()

	if cfg.WorkoutDataSync != Default().WorkoutDataSync {
		t.Fatalf("fallback config = %+v, want %+v", cfg.WorkoutDataSync, Default().WorkoutDataSync)
	}
}

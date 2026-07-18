package mysqlmodel

import (
	"errors"
	"fmt"
	"strings"
	"time"

	appconfig "spider-server/common/config"
	pb "spider-server/gen/spider/api"
	"spider-server/mysql/config"

	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
)

var (
	maxWorkoutDataSnapshotPayloadBytes        = appconfig.Default().WorkoutDataSync.SnapshotMaxPayloadBytes
	workoutDataSnapshotRestoreBatchBytes      = appconfig.Default().WorkoutDataSync.RestoreBatchTargetBytes
	maxWorkoutDataSnapshotRestoreBatchRecords = appconfig.Default().WorkoutDataSync.RestoreBatchMaxSnapshots
)

func ConfigureWorkoutDataSnapshotLimits(cfg appconfig.WorkoutDataSyncConfig) {
	defaults := appconfig.Default().WorkoutDataSync
	if cfg.SnapshotMaxPayloadBytes <= 0 {
		cfg.SnapshotMaxPayloadBytes = defaults.SnapshotMaxPayloadBytes
	}
	if cfg.RestoreBatchTargetBytes <= 0 {
		cfg.RestoreBatchTargetBytes = defaults.RestoreBatchTargetBytes
	}
	if cfg.RestoreBatchMaxSnapshots <= 0 {
		cfg.RestoreBatchMaxSnapshots = defaults.RestoreBatchMaxSnapshots
	}
	maxWorkoutDataSnapshotPayloadBytes = cfg.SnapshotMaxPayloadBytes
	workoutDataSnapshotRestoreBatchBytes = cfg.RestoreBatchTargetBytes
	maxWorkoutDataSnapshotRestoreBatchRecords = cfg.RestoreBatchMaxSnapshots
}

// WorkoutDataSnapshot stores the latest payload or tombstone for one workout entity.
// New plan data uses folder/plan entities; the fixed library entity remains for legacy restore compatibility.
type WorkoutDataSnapshot struct {
	ID               uint64         `gorm:"primaryKey;autoIncrement;index:idx_uid_workout_snapshot_server_changed,priority:3"`
	UID              uint64         `gorm:"not null;uniqueIndex:idx_uid_workout_snapshot_entity,priority:1;index:idx_uid_workout_snapshot_server_changed,priority:1"`
	Kind             int32          `gorm:"not null;uniqueIndex:idx_uid_workout_snapshot_entity,priority:2"`
	EntityID         string         `gorm:"type:varchar(64);not null;uniqueIndex:idx_uid_workout_snapshot_entity,priority:3"`
	ClientSnapshotID string         `gorm:"type:varchar(64);not null;default:''"`
	ClientChangedAt  int64          `gorm:"not null;default:0"`
	Payload          []byte         `gorm:"type:longblob;not null"`
	CreatedAt        time.Time      `gorm:"not null"`
	UpdatedAt        time.Time      `gorm:"not null"`
	DeletedAt        gorm.DeletedAt `gorm:"index"`
	ServerChangedAt  *time.Time     `gorm:"type:datetime(3);index:idx_uid_workout_snapshot_server_changed,priority:2"`
}

func SaveWorkoutDataSnapshots(uid uint64, snapshots []*pb.WorkoutDataSnapshot) ([]string, error) {
	if uid == 0 {
		return nil, fmt.Errorf("uid is empty")
	}
	if len(snapshots) == 0 {
		return nil, fmt.Errorf("snapshots are empty")
	}

	db, err := config.DB()
	if err != nil {
		return nil, err
	}

	accepted := make([]string, 0, len(snapshots))
	err = db.Transaction(func(tx *gorm.DB) error {
		for _, snapshot := range snapshots {
			if snapshot == nil {
				return fmt.Errorf("snapshot is nil")
			}
			clientSnapshotID := strings.TrimSpace(snapshot.GetClientSnapshotId())
			entityID := strings.TrimSpace(snapshot.GetEntityId())
			if clientSnapshotID == "" || entityID == "" {
				return fmt.Errorf("snapshot identity is empty")
			}
			if snapshot.GetKind() != pb.WorkoutDataSnapshotKind_WORKOUT_DATA_SNAPSHOT_KIND_LIBRARY &&
				snapshot.GetKind() != pb.WorkoutDataSnapshotKind_WORKOUT_DATA_SNAPSHOT_KIND_TRAINING_SESSION {
				return fmt.Errorf("snapshot kind is invalid")
			}

			changedAt := snapshot.GetChangedAt()
			if changedAt <= 0 {
				changedAt = time.Now().UnixMilli()
			}
			storedPB := proto.Clone(snapshot).(*pb.WorkoutDataSnapshot)
			storedPB.Id = 0
			storedPB.Uid = uid
			storedPB.ClientSnapshotId = clientSnapshotID
			storedPB.EntityId = entityID
			storedPB.ChangedAt = changedAt
			storedPB.CreatedAt = 0
			storedPB.UpdatedAt = 0
			payload, marshalErr := proto.Marshal(storedPB)
			if marshalErr != nil {
				return marshalErr
			}
			if int64(len(payload)) > maxWorkoutDataSnapshotPayloadBytes {
				return fmt.Errorf("snapshot payload is too large")
			}
			serverChangedAt := time.Now()
			deletedAt := gorm.DeletedAt{}
			var deletedAtUpdate any
			if storedPB.GetDeleted() {
				deletedAt = gorm.DeletedAt{Time: serverChangedAt, Valid: true}
				deletedAtUpdate = serverChangedAt
			}

			var existing WorkoutDataSnapshot
			findErr := tx.Unscoped().Where(
				"uid = ? AND kind = ? AND entity_id = ?",
				uid,
				int32(snapshot.GetKind()),
				entityID,
			).First(&existing).Error
			if findErr != nil && !errors.Is(findErr, gorm.ErrRecordNotFound) {
				return findErr
			}
			if errors.Is(findErr, gorm.ErrRecordNotFound) {
				record := &WorkoutDataSnapshot{
					UID:              uid,
					Kind:             int32(snapshot.GetKind()),
					EntityID:         entityID,
					ClientSnapshotID: clientSnapshotID,
					ClientChangedAt:  changedAt,
					Payload:          payload,
					DeletedAt:        deletedAt,
					ServerChangedAt:  &serverChangedAt,
				}
				if createErr := tx.Create(record).Error; createErr != nil {
					return createErr
				}
			} else if existing.ClientChangedAt <= changedAt {
				if updateErr := tx.Unscoped().Model(&existing).Updates(map[string]any{
					"client_snapshot_id": clientSnapshotID,
					"client_changed_at":  changedAt,
					"payload":            payload,
					"deleted_at":         deletedAtUpdate,
					"updated_at":         serverChangedAt,
					"server_changed_at":  serverChangedAt,
				}).Error; updateErr != nil {
					return updateErr
				}
			}
			accepted = append(accepted, clientSnapshotID)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return accepted, nil
}

func CountWorkoutDataSnapshotChanges(uid uint64, startSnapshotID int64, endSnapshotID int64) (uint64, error) {
	if uid == 0 {
		return 0, fmt.Errorf("uid is empty")
	}
	if endSnapshotID <= 0 || startSnapshotID > endSnapshotID {
		return 0, fmt.Errorf("snapshot range is invalid")
	}
	db, err := config.DB()
	if err != nil {
		return 0, err
	}
	var count int64
	if err := workoutDataSnapshotChangesQuery(db, uid, startSnapshotID, endSnapshotID).
		Model(&WorkoutDataSnapshot{}).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return uint64(count), nil
}

func ListWorkoutDataSnapshotChangesPage(uid uint64, startSnapshotID int64, endSnapshotID int64, limit int, offset int) ([]*WorkoutDataSnapshot, error) {
	if uid == 0 {
		return nil, fmt.Errorf("uid is empty")
	}
	if endSnapshotID <= 0 || startSnapshotID > endSnapshotID {
		return nil, fmt.Errorf("snapshot range is invalid")
	}
	if limit <= 0 {
		limit = DefaultExerciseSetPageSize
	}
	if offset < 0 {
		offset = 0
	}
	db, err := config.DB()
	if err != nil {
		return nil, err
	}
	var records []*WorkoutDataSnapshot
	if err := workoutDataSnapshotChangesQuery(db, uid, startSnapshotID, endSnapshotID).
		Order(workoutDataSnapshotChangedAtSQL() + " ASC, id ASC").
		Limit(limit).
		Offset(offset).
		Find(&records).Error; err != nil {
		return nil, err
	}
	return records, nil
}

func ListWorkoutDataSnapshotChangesByByteCursor(
	uid uint64,
	startSnapshotID int64,
	endSnapshotID int64,
	cursorServerChangedAt int64,
	cursorID uint64,
) ([]*WorkoutDataSnapshot, bool, error) {
	if uid == 0 {
		return nil, false, fmt.Errorf("uid is empty")
	}
	if endSnapshotID <= 0 || startSnapshotID > endSnapshotID {
		return nil, false, fmt.Errorf("snapshot range is invalid")
	}
	db, err := config.DB()
	if err != nil {
		return nil, false, err
	}

	query := workoutDataSnapshotChangesQuery(db, uid, startSnapshotID, endSnapshotID)
	if cursorServerChangedAt > 0 {
		cursorTime := time.UnixMilli(cursorServerChangedAt)
		query = query.Where(
			"(server_changed_at > ? OR (server_changed_at = ? AND id > ?))",
			cursorTime,
			cursorTime,
			cursorID,
		)
	}
	rows, err := query.Model(&WorkoutDataSnapshot{}).Order("server_changed_at ASC, id ASC").Rows()
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	records := make([]*WorkoutDataSnapshot, 0)
	var totalPayloadBytes int64
	hasMore := false
	for rows.Next() {
		if len(records) >= maxWorkoutDataSnapshotRestoreBatchRecords {
			hasMore = true
			break
		}
		var record WorkoutDataSnapshot
		if err := db.ScanRows(rows, &record); err != nil {
			return nil, false, err
		}
		payloadBytes := int64(len(record.Payload))
		if len(records) > 0 && totalPayloadBytes+payloadBytes > workoutDataSnapshotRestoreBatchBytes {
			hasMore = true
			break
		}
		records = append(records, &record)
		totalPayloadBytes += payloadBytes
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	return records, hasMore, nil
}

func BackfillWorkoutDataSnapshotServerChangedAt() error {
	db, err := config.DB()
	if err != nil {
		return err
	}
	return db.Unscoped().Model(&WorkoutDataSnapshot{}).
		Where("server_changed_at IS NULL").
		Update("server_changed_at", gorm.Expr("GREATEST(updated_at, COALESCE(deleted_at, updated_at))")).Error
}

func WorkoutDataSnapshotToPB(record *WorkoutDataSnapshot) (*pb.WorkoutDataSnapshot, error) {
	if record == nil {
		return nil, fmt.Errorf("snapshot is nil")
	}
	snapshot := &pb.WorkoutDataSnapshot{}
	if err := proto.Unmarshal(record.Payload, snapshot); err != nil {
		return nil, err
	}
	snapshot.Id = record.ID
	snapshot.Uid = record.UID
	snapshot.ClientSnapshotId = record.ClientSnapshotID
	snapshot.Kind = pb.WorkoutDataSnapshotKind(record.Kind)
	snapshot.EntityId = record.EntityID
	snapshot.ChangedAt = record.ClientChangedAt
	snapshot.Deleted = record.DeletedAt.Valid
	snapshot.CreatedAt = record.CreatedAt.UnixMilli()
	snapshot.UpdatedAt = record.UpdatedAt.UnixMilli()
	return snapshot, nil
}

func workoutDataSnapshotChangesQuery(db *gorm.DB, uid uint64, startSnapshotID int64, endSnapshotID int64) *gorm.DB {
	endTime := time.UnixMilli(endSnapshotID)
	query := db.Unscoped().Where("uid = ?", uid)
	if startSnapshotID <= 0 {
		return query.Where("server_changed_at <= ?", endTime)
	}
	startTime := time.UnixMilli(startSnapshotID)
	return query.Where("server_changed_at > ? AND server_changed_at <= ?", startTime, endTime)
}

func workoutDataSnapshotChangedAtSQL() string {
	return "server_changed_at"
}

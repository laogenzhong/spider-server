package mysqlmodel

import (
	"errors"
	"fmt"
	"time"

	"spider-server/mysql/config"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// BodyPhotoRecord 表示 App 内照片索引记录。
//
// 这里只保存照片库 localIdentifier 和展示元数据，不保存图片二进制。
type BodyPhotoRecord struct {
	ID                  uint64         `gorm:"primaryKey;autoIncrement"`
	UID                 uint64         `gorm:"not null;index;uniqueIndex:idx_uid_client_photo_record"`
	ClientRecordID      string         `gorm:"type:varchar(128);not null;uniqueIndex:idx_uid_client_photo_record"`
	PhotoLibraryAssetID string         `gorm:"type:varchar(256);not null;index"`
	Kind                int32          `gorm:"not null;index"`
	RecordAt            int64          `gorm:"not null;index"`
	Weight              float64        `gorm:"not null;default:0"`
	Note                string         `gorm:"type:varchar(512);not null;default:''"`
	FileName            string         `gorm:"type:varchar(256);not null;default:''"`
	CreatedAt           time.Time      `gorm:"not null"`
	UpdatedAt           time.Time      `gorm:"not null"`
	DeletedAt           gorm.DeletedAt `gorm:"index"`
}

const (
	BodyPhotoKindBody int32 = 1
	BodyPhotoKindDiet int32 = 2

	MaxBodyPhotosPerDay       = 100
	MaxBodyPhotoCreatesPerDay = 500
)

var (
	ErrBodyPhotoDailyLimitExceeded       = errors.New("body photo daily limit exceeded")
	ErrBodyPhotoDailyCreateLimitExceeded = errors.New("body photo daily create limit exceeded")
)

// SaveBodyPhotoRecord 创建或更新照片索引记录。
func SaveBodyPhotoRecord(record *BodyPhotoRecord) (*BodyPhotoRecord, error) {
	if record == nil {
		return nil, fmt.Errorf("record is nil")
	}
	if record.UID == 0 {
		return nil, fmt.Errorf("uid is empty")
	}
	if record.ClientRecordID == "" {
		return nil, fmt.Errorf("client_record_id is empty")
	}
	if record.PhotoLibraryAssetID == "" {
		return nil, fmt.Errorf("photo_library_asset_id is empty")
	}
	if record.Kind == 0 {
		return nil, fmt.Errorf("kind is empty")
	}
	if record.RecordAt == 0 {
		return nil, fmt.Errorf("record_at is empty")
	}

	db, err := config.DB()
	if err != nil {
		return nil, err
	}

	err = db.Transaction(func(tx *gorm.DB) error {
		existing := &BodyPhotoRecord{}
		existingID := uint64(0)
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Unscoped().
			Where("uid = ? AND client_record_id = ?", record.UID, record.ClientRecordID).
			First(existing).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err == nil {
			existingID = existing.ID
		}

		if existingID == 0 {
			totalCount, err := countBodyPhotoRecordsOnDayUnscoped(tx, record.UID, record.RecordAt)
			if err != nil {
				return err
			}
			if totalCount >= MaxBodyPhotoCreatesPerDay {
				return ErrBodyPhotoDailyCreateLimitExceeded
			}
		}

		count, err := countBodyPhotoRecordsOnDay(tx, record.UID, record.RecordAt, existingID)
		if err != nil {
			return err
		}
		if count >= MaxBodyPhotosPerDay {
			return ErrBodyPhotoDailyLimitExceeded
		}

		return tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "uid"},
				{Name: "client_record_id"},
			},
			DoUpdates: clause.Assignments(map[string]any{
				"photo_library_asset_id": record.PhotoLibraryAssetID,
				"kind":                   record.Kind,
				"record_at":              record.RecordAt,
				"weight":                 record.Weight,
				"note":                   record.Note,
				"file_name":              record.FileName,
				"deleted_at":             nil,
				"updated_at":             time.Now(),
			}),
		}).Create(record).Error
	})
	if err != nil {
		return nil, err
	}

	return GetBodyPhotoRecordByClientID(record.UID, record.ClientRecordID)
}

func countBodyPhotoRecordsOnDay(tx *gorm.DB, uid uint64, recordAt int64, excludeID uint64) (int, error) {
	startAt, endAt := dayBoundsMillis(recordAt)
	query := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("uid = ? AND record_at >= ? AND record_at < ?", uid, startAt, endAt)
	if excludeID > 0 {
		query = query.Where("id <> ?", excludeID)
	}

	var records []*BodyPhotoRecord
	if err := query.Find(&records).Error; err != nil {
		return 0, err
	}
	return len(records), nil
}

func countBodyPhotoRecordsOnDayUnscoped(tx *gorm.DB, uid uint64, recordAt int64) (int, error) {
	startAt, endAt := dayBoundsMillis(recordAt)
	var records []*BodyPhotoRecord
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Unscoped().
		Where("uid = ? AND record_at >= ? AND record_at < ?", uid, startAt, endAt).
		Find(&records).Error; err != nil {
		return 0, err
	}
	return len(records), nil
}

func dayBoundsMillis(millis int64) (int64, int64) {
	t := time.UnixMilli(millis)
	start := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	return start.UnixMilli(), start.AddDate(0, 0, 1).UnixMilli()
}

// GetBodyPhotoRecordByClientID 根据客户端记录 ID 查询照片索引。
func GetBodyPhotoRecordByClientID(uid uint64, clientRecordID string) (*BodyPhotoRecord, error) {
	if uid == 0 {
		return nil, fmt.Errorf("uid is empty")
	}
	if clientRecordID == "" {
		return nil, fmt.Errorf("client_record_id is empty")
	}

	db, err := config.DB()
	if err != nil {
		return nil, err
	}

	record := &BodyPhotoRecord{}
	if err := db.Where("uid = ? AND client_record_id = ?", uid, clientRecordID).First(record).Error; err != nil {
		return nil, err
	}
	return record, nil
}

// DeleteBodyPhotoRecord 删除照片索引，优先按服务端 id 删除，id 为空时按 client_record_id 删除。
func DeleteBodyPhotoRecord(uid uint64, id uint64, clientRecordID string) error {
	if uid == 0 {
		return fmt.Errorf("uid is empty")
	}
	if id == 0 && clientRecordID == "" {
		return fmt.Errorf("id and client_record_id are both empty")
	}

	db, err := config.DB()
	if err != nil {
		return err
	}

	query := db.Where("uid = ?", uid)
	if id > 0 {
		query = query.Where("id = ?", id)
	} else {
		query = query.Where("client_record_id = ?", clientRecordID)
	}

	result := query.Delete(&BodyPhotoRecord{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}

// CountBodyPhotoRecordChanges 统计指定快照窗口内需要同步的照片索引数量和日期范围。
func CountBodyPhotoRecordChanges(uid uint64, startSnapshotID int64, endSnapshotID int64) (uint64, string, string, error) {
	if uid == 0 {
		return 0, "", "", fmt.Errorf("uid is empty")
	}
	if endSnapshotID <= 0 {
		return 0, "", "", fmt.Errorf("end_snapshot_id is empty")
	}

	db, err := config.DB()
	if err != nil {
		return 0, "", "", err
	}

	var count int64
	if err := bodyPhotoRecordChangesQuery(db, uid, startSnapshotID, endSnapshotID).
		Model(&BodyPhotoRecord{}).
		Count(&count).Error; err != nil {
		return 0, "", "", err
	}
	if count == 0 {
		return 0, "", "", nil
	}

	var bounds struct {
		StartAt int64
		EndAt   int64
	}
	if err := bodyPhotoRecordChangesQuery(db, uid, startSnapshotID, endSnapshotID).
		Model(&BodyPhotoRecord{}).
		Select("MIN(record_at) AS start_at, MAX(record_at) AS end_at").
		Scan(&bounds).Error; err != nil {
		return 0, "", "", err
	}

	return uint64(count), recordDateFromMillis(bounds.StartAt), recordDateFromMillis(bounds.EndAt), nil
}

// ListBodyPhotoRecordChangesPage 分页查询指定快照窗口内需要同步的照片索引。
func ListBodyPhotoRecordChangesPage(uid uint64, startSnapshotID int64, endSnapshotID int64, limit int, offset int) ([]*BodyPhotoRecord, error) {
	if uid == 0 {
		return nil, fmt.Errorf("uid is empty")
	}
	if endSnapshotID <= 0 {
		return nil, fmt.Errorf("end_snapshot_id is empty")
	}
	if limit <= 0 {
		return nil, fmt.Errorf("limit must be greater than 0")
	}
	if offset < 0 {
		return nil, fmt.Errorf("offset must be greater than or equal to 0")
	}

	db, err := config.DB()
	if err != nil {
		return nil, err
	}

	var records []*BodyPhotoRecord
	if err := bodyPhotoRecordChangesQuery(db, uid, startSnapshotID, endSnapshotID).
		Order(bodyPhotoRecordChangedAtSQL() + " ASC, record_at ASC, id ASC").
		Limit(limit).
		Offset(offset).
		Find(&records).Error; err != nil {
		return nil, err
	}
	return records, nil
}

func bodyPhotoRecordChangesQuery(db *gorm.DB, uid uint64, startSnapshotID int64, endSnapshotID int64) *gorm.DB {
	endTime := time.UnixMilli(endSnapshotID)
	query := db.Unscoped().Where("uid = ?", uid)
	if startSnapshotID <= 0 {
		return query.Where("created_at <= ? AND (deleted_at IS NULL OR deleted_at > ?)", endTime, endTime)
	}

	startTime := time.UnixMilli(startSnapshotID)
	return query.Where(
		"(created_at > ? AND created_at <= ?) OR (updated_at > ? AND updated_at <= ?) OR (deleted_at IS NOT NULL AND deleted_at > ? AND deleted_at <= ?)",
		startTime,
		endTime,
		startTime,
		endTime,
		startTime,
		endTime,
	)
}

func bodyPhotoRecordChangedAtSQL() string {
	return "GREATEST(updated_at, COALESCE(deleted_at, updated_at))"
}

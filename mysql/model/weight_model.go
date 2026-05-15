package mysqlmodel

import (
	"fmt"
	"spider-server/mysql/config"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// WeightRecord 表示用户每天的体重和摄入状态记录。
//
// 设计说明：
// 1. UID + RecordDate 唯一，保证同一用户同一天只有一条体重记录。
// 2. Weight 使用 kg * 10 的整数，例如 70.5kg 存为 705，避免浮点误差。
// 3. Satiety 是摄入状态评分，范围 0-10，不是具体 kcal。
// 4. 服务端 MySQL 是主数据源，客户端新增、修改、删除以服务端成功为准。
type WeightRecord struct {
	ID         uint64         `gorm:"primaryKey;autoIncrement"`
	UID        uint64         `gorm:"not null;uniqueIndex:idx_uid_record_date"`
	RecordDate string         `gorm:"type:varchar(20);not null;uniqueIndex:idx_uid_record_date"`
	Weight     int32          `gorm:"not null"`
	Satiety    int32          `gorm:"not null"`
	CreatedAt  time.Time      `gorm:"not null"`
	UpdatedAt  time.Time      `gorm:"not null"`
	DeletedAt  gorm.DeletedAt `gorm:"index"`
}

// CreateWeightRecord 创建或覆盖某一天的体重记录。
//
// 如果同一个 uid + record_date 已经存在，则更新 weight 和 satiety。
func CreateWeightRecord(uid uint64, recordDate string, weight int32, satiety int32) (*WeightRecord, error) {
	if uid == 0 {
		return nil, fmt.Errorf("uid is empty")
	}
	if recordDate == "" {
		return nil, fmt.Errorf("record_date is empty")
	}

	record := &WeightRecord{
		UID:        uid,
		RecordDate: recordDate,
		Weight:     weight,
		Satiety:    satiety,
	}

	db, err := config.DB()
	if err != nil {
		return nil, err
	}

	err = db.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "uid"},
			{Name: "record_date"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"weight",
			"satiety",
			"updated_at",
		}),
	}).Create(record).Error
	if err != nil {
		return nil, err
	}

	return GetWeightRecordByDate(uid, recordDate)
}

// UpdateWeightRecord 修改体重记录。
//
// 优先使用 id 定位；id 为空时，使用 uid + record_date 定位。
func UpdateWeightRecord(uid uint64, id uint64, recordDate string, weight int32, satiety int32) (*WeightRecord, error) {
	if uid == 0 {
		return nil, fmt.Errorf("uid is empty")
	}
	if id == 0 && recordDate == "" {
		return nil, fmt.Errorf("id and record_date are both empty")
	}

	db, err := config.DB()
	if err != nil {
		return nil, err
	}

	query := db.Model(&WeightRecord{}).Where("uid = ?", uid)
	if id > 0 {
		query = query.Where("id = ?", id)
	} else {
		query = query.Where("record_date = ?", recordDate)
	}

	result := query.Updates(map[string]any{
		"weight":  weight,
		"satiety": satiety,
	})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	if id > 0 {
		return GetWeightRecordByID(uid, id)
	}
	return GetWeightRecordByDate(uid, recordDate)
}

// GetWeightRecordByID 根据 id 查询体重记录。
func GetWeightRecordByID(uid uint64, id uint64) (*WeightRecord, error) {
	if uid == 0 {
		return nil, fmt.Errorf("uid is empty")
	}
	if id == 0 {
		return nil, fmt.Errorf("id is empty")
	}

	record := &WeightRecord{}
	db, err := config.DB()
	if err != nil {
		return nil, err
	}
	if err := db.Where("uid = ? AND id = ?", uid, id).First(record).Error; err != nil {
		return nil, err
	}
	return record, nil
}

// GetWeightRecordByDate 根据日期查询体重记录。
func GetWeightRecordByDate(uid uint64, recordDate string) (*WeightRecord, error) {
	if uid == 0 {
		return nil, fmt.Errorf("uid is empty")
	}
	if recordDate == "" {
		return nil, fmt.Errorf("record_date is empty")
	}

	record := &WeightRecord{}
	db, err := config.DB()
	if err != nil {
		return nil, err
	}
	if err := db.Where("uid = ? AND record_date = ?", uid, recordDate).First(record).Error; err != nil {
		return nil, err
	}
	return record, nil
}

// ListWeightRecords 查询日期范围内的体重记录。
func ListWeightRecords(uid uint64, startDate string, endDate string) ([]*WeightRecord, error) {
	if uid == 0 {
		return nil, fmt.Errorf("uid is empty")
	}
	if startDate == "" || endDate == "" {
		return nil, fmt.Errorf("start_date and end_date are required")
	}

	var records []*WeightRecord
	db, err := config.DB()
	if err != nil {
		return nil, err
	}

	err = db.Where("uid = ? AND record_date >= ? AND record_date <= ?", uid, startDate, endDate).
		Order("record_date ASC").
		Find(&records).
		Error
	if err != nil {
		return nil, err
	}

	return records, nil
}

// CountWeightRecords 统计当前用户体重记录总数和日期范围。
func CountWeightRecords(uid uint64) (uint64, string, string, error) {
	if uid == 0 {
		return 0, "", "", fmt.Errorf("uid is empty")
	}

	db, err := config.DB()
	if err != nil {
		return 0, "", "", err
	}

	var count int64
	if err := db.Model(&WeightRecord{}).Where("uid = ?", uid).Count(&count).Error; err != nil {
		return 0, "", "", err
	}
	if count == 0 {
		return 0, "", "", nil
	}

	var bounds struct {
		StartDate string
		EndDate   string
	}
	if err := db.Model(&WeightRecord{}).
		Select("MIN(record_date) AS start_date, MAX(record_date) AS end_date").
		Where("uid = ?", uid).
		Scan(&bounds).Error; err != nil {
		return 0, "", "", err
	}

	return uint64(count), bounds.StartDate, bounds.EndDate, nil
}

// ListWeightRecordsPage 分页查询当前用户体重记录，用于客户端全量恢复。
func ListWeightRecordsPage(uid uint64, limit int, offset int) ([]*WeightRecord, error) {
	if uid == 0 {
		return nil, fmt.Errorf("uid is empty")
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

	var records []*WeightRecord
	if err := db.Where("uid = ?", uid).
		Order("record_date ASC, id ASC").
		Limit(limit).
		Offset(offset).
		Find(&records).Error; err != nil {
		return nil, err
	}
	return records, nil
}

// CountWeightRecordChanges 统计指定快照窗口内需要同步的体重记录数量和日期范围。
func CountWeightRecordChanges(uid uint64, startSnapshotID int64, endSnapshotID int64) (uint64, string, string, error) {
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
	if err := weightRecordChangesQuery(db, uid, startSnapshotID, endSnapshotID).
		Model(&WeightRecord{}).
		Count(&count).Error; err != nil {
		return 0, "", "", err
	}
	if count == 0 {
		return 0, "", "", nil
	}

	var bounds struct {
		StartDate string
		EndDate   string
	}
	if err := weightRecordChangesQuery(db, uid, startSnapshotID, endSnapshotID).
		Model(&WeightRecord{}).
		Select("MIN(record_date) AS start_date, MAX(record_date) AS end_date").
		Scan(&bounds).Error; err != nil {
		return 0, "", "", err
	}

	return uint64(count), bounds.StartDate, bounds.EndDate, nil
}

// ListWeightRecordChangesPage 分页查询指定快照窗口内需要同步的体重记录。
func ListWeightRecordChangesPage(uid uint64, startSnapshotID int64, endSnapshotID int64, limit int, offset int) ([]*WeightRecord, error) {
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

	var records []*WeightRecord
	if err := weightRecordChangesQuery(db, uid, startSnapshotID, endSnapshotID).
		Order(weightRecordChangedAtSQL() + " ASC, id ASC").
		Limit(limit).
		Offset(offset).
		Find(&records).Error; err != nil {
		return nil, err
	}
	return records, nil
}

func weightRecordChangesQuery(db *gorm.DB, uid uint64, startSnapshotID int64, endSnapshotID int64) *gorm.DB {
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

func weightRecordChangedAtSQL() string {
	return "GREATEST(updated_at, COALESCE(deleted_at, updated_at))"
}

// GetLatestWeightRecord 查询当前用户最近一条体重记录。
func GetLatestWeightRecord(uid uint64) (*WeightRecord, error) {
	if uid == 0 {
		return nil, fmt.Errorf("uid is empty")
	}

	record := &WeightRecord{}
	db, err := config.DB()
	if err != nil {
		return nil, err
	}

	if err := db.Where("uid = ?", uid).
		Order("record_date DESC, updated_at DESC, id DESC").
		First(record).
		Error; err != nil {
		return nil, err
	}

	return record, nil
}

// DeleteWeightRecord 删除体重记录。
//
// 优先使用 id 删除；id 为空时，使用 uid + record_date 删除。
func DeleteWeightRecord(uid uint64, id uint64, recordDate string) error {
	if uid == 0 {
		return fmt.Errorf("uid is empty")
	}
	if id == 0 && recordDate == "" {
		return fmt.Errorf("id and record_date are both empty")
	}

	db, err := config.DB()
	if err != nil {
		return err
	}

	query := db.Where("uid = ?", uid)
	if id > 0 {
		query = query.Where("id = ?", id)
	} else {
		query = query.Where("record_date = ?", recordDate)
	}

	result := query.Delete(&WeightRecord{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}

package mysqlmodel

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	pb "spider-server/gen/spider/api"
	"spider-server/mysql/config"

	"gorm.io/gorm"
)

// ExerciseSetRecord 表示动作详情页中记录的一组重量和次数。
type ExerciseSetRecord struct {
	ID                   uint64         `gorm:"primaryKey;autoIncrement"`
	UID                  uint64         `gorm:"not null;index:idx_uid_exercise_recorded,priority:1;index:idx_uid_recorded,priority:1"`
	ExerciseID           string         `gorm:"type:varchar(128);not null;index:idx_uid_exercise_recorded,priority:2"`
	ExerciseNameKey      string         `gorm:"type:varchar(128);not null;default:''"`
	ExerciseNameSnapshot string         `gorm:"type:varchar(256);not null;default:''"`
	CategoryKey          string         `gorm:"type:varchar(128);not null;default:''"`
	TypeKey              string         `gorm:"type:varchar(128);not null;default:''"`
	WeightX10            int32          `gorm:"not null;default:0"`
	WeightUnit           int32          `gorm:"not null;default:0"`
	Reps                 int32          `gorm:"not null;default:0"`
	RecordedAt           int64          `gorm:"not null;index:idx_uid_exercise_recorded,priority:3;index:idx_uid_recorded,priority:2"`
	CreatedAt            time.Time      `gorm:"not null"`
	UpdatedAt            time.Time      `gorm:"not null"`
	DeletedAt            gorm.DeletedAt `gorm:"index"`
}

const (
	MaxExerciseSetRecordPageSize = 20
	DefaultExerciseSetPageSize   = 20
	TodayExerciseLatestLimit     = 3
)

// TodayExerciseHistory 表示今日动作快捷入口的聚合数据。
type TodayExerciseHistory struct {
	ExerciseID           string
	ExerciseNameKey      string
	ExerciseNameSnapshot string
	CategoryKey          string
	TypeKey              string
	SetCount             int32
	MaxWeightX10         int32
	WeightUnit           int32
	LatestRecordedAt     int64
	LatestRecords        []*ExerciseSetRecord
}

// CreateExerciseSetRecord 创建一组动作记录。
func CreateExerciseSetRecord(record *ExerciseSetRecord) (*ExerciseSetRecord, error) {
	if record == nil {
		return nil, fmt.Errorf("record is nil")
	}
	if record.UID == 0 {
		return nil, fmt.Errorf("uid is empty")
	}
	if record.ExerciseID == "" {
		return nil, fmt.Errorf("exercise_id is empty")
	}
	if record.WeightX10 < 0 {
		return nil, fmt.Errorf("weight_x10 is invalid")
	}
	if record.Reps <= 0 {
		return nil, fmt.Errorf("reps is invalid")
	}
	if record.RecordedAt <= 0 {
		record.RecordedAt = time.Now().UnixMilli()
	}

	db, err := config.DB()
	if err != nil {
		return nil, err
	}
	if err := db.Create(record).Error; err != nil {
		return nil, err
	}
	return GetExerciseSetRecordByID(record.UID, record.ID)
}

// UpdateExerciseSetRecord 修改一组动作记录。
func UpdateExerciseSetRecord(record *ExerciseSetRecord) (*ExerciseSetRecord, error) {
	if record == nil {
		return nil, fmt.Errorf("record is nil")
	}
	if record.UID == 0 {
		return nil, fmt.Errorf("uid is empty")
	}
	if record.ID == 0 {
		return nil, fmt.Errorf("id is empty")
	}
	if record.WeightX10 < 0 {
		return nil, fmt.Errorf("weight_x10 is invalid")
	}
	if record.Reps <= 0 {
		return nil, fmt.Errorf("reps is invalid")
	}
	if record.RecordedAt <= 0 {
		record.RecordedAt = time.Now().UnixMilli()
	}

	db, err := config.DB()
	if err != nil {
		return nil, err
	}

	updates := map[string]any{
		"exercise_name_key":      record.ExerciseNameKey,
		"exercise_name_snapshot": record.ExerciseNameSnapshot,
		"category_key":           record.CategoryKey,
		"type_key":               record.TypeKey,
		"weight_x10":             record.WeightX10,
		"weight_unit":            record.WeightUnit,
		"reps":                   record.Reps,
		"recorded_at":            record.RecordedAt,
	}
	result := db.Model(&ExerciseSetRecord{}).
		Where("uid = ? AND id = ?", record.UID, record.ID).
		Updates(updates)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return GetExerciseSetRecordByID(record.UID, record.ID)
}

// DeleteExerciseSetRecord 删除一组动作记录。
func DeleteExerciseSetRecord(uid uint64, id uint64) error {
	if uid == 0 {
		return fmt.Errorf("uid is empty")
	}
	if id == 0 {
		return fmt.Errorf("id is empty")
	}

	db, err := config.DB()
	if err != nil {
		return err
	}
	result := db.Where("uid = ? AND id = ?", uid, id).Delete(&ExerciseSetRecord{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// GetExerciseSetRecordByID 根据 id 查询动作记录。
func GetExerciseSetRecordByID(uid uint64, id uint64) (*ExerciseSetRecord, error) {
	if uid == 0 {
		return nil, fmt.Errorf("uid is empty")
	}
	if id == 0 {
		return nil, fmt.Errorf("id is empty")
	}

	db, err := config.DB()
	if err != nil {
		return nil, err
	}
	record := &ExerciseSetRecord{}
	if err := db.Where("uid = ? AND id = ?", uid, id).First(record).Error; err != nil {
		return nil, err
	}
	return record, nil
}

// ListExerciseSetRecords 分页查询某个动作的记录，按记录时间倒序。
func ListExerciseSetRecords(uid uint64, exerciseID string, pageSize int32, cursor string) ([]*ExerciseSetRecord, string, bool, error) {
	if uid == 0 {
		return nil, "", false, fmt.Errorf("uid is empty")
	}
	if exerciseID == "" {
		return nil, "", false, fmt.Errorf("exercise_id is empty")
	}

	limit := normalizeExerciseRecordPageSize(pageSize)
	cursorRecordedAt, cursorID, err := parseExerciseRecordCursor(cursor)
	if err != nil {
		return nil, "", false, err
	}

	db, err := config.DB()
	if err != nil {
		return nil, "", false, err
	}

	query := db.Where("uid = ? AND exercise_id = ?", uid, exerciseID)
	if cursorRecordedAt > 0 && cursorID > 0 {
		query = query.Where("recorded_at < ? OR (recorded_at = ? AND id < ?)", cursorRecordedAt, cursorRecordedAt, cursorID)
	}

	var records []*ExerciseSetRecord
	if err := query.
		Order("recorded_at DESC, id DESC").
		Limit(limit + 1).
		Find(&records).Error; err != nil {
		return nil, "", false, err
	}

	hasMore := len(records) > limit
	if hasMore {
		records = records[:limit]
	}

	nextCursor := ""
	if hasMore && len(records) > 0 {
		last := records[len(records)-1]
		nextCursor = formatExerciseRecordCursor(last.RecordedAt, last.ID)
	}

	return records, nextCursor, hasMore, nil
}

// ListExerciseSetRecordsByTimeRange 按记录时间范围查询动作记录，按记录时间倒序。
func ListExerciseSetRecordsByTimeRange(uid uint64, startAt int64, endAt int64) ([]*ExerciseSetRecord, error) {
	if uid == 0 {
		return nil, fmt.Errorf("uid is empty")
	}
	if startAt <= 0 || endAt <= 0 || startAt > endAt {
		return nil, fmt.Errorf("time range is invalid")
	}

	db, err := config.DB()
	if err != nil {
		return nil, err
	}

	var records []*ExerciseSetRecord
	if err := db.Where("uid = ? AND recorded_at >= ? AND recorded_at <= ?", uid, startAt, endAt).
		Order("recorded_at DESC, id DESC").
		Find(&records).Error; err != nil {
		return nil, err
	}

	return records, nil
}

// ListTodayExerciseHistory 查询某天的动作记录聚合。
func ListTodayExerciseHistory(uid uint64, recordDate string) ([]*TodayExerciseHistory, error) {
	if uid == 0 {
		return nil, fmt.Errorf("uid is empty")
	}

	startAt, endAt, err := exerciseRecordDayBounds(recordDate)
	if err != nil {
		return nil, err
	}

	db, err := config.DB()
	if err != nil {
		return nil, err
	}

	var records []*ExerciseSetRecord
	if err := db.Where("uid = ? AND recorded_at >= ? AND recorded_at < ?", uid, startAt, endAt).
		Order("recorded_at DESC, id DESC").
		Find(&records).Error; err != nil {
		return nil, err
	}

	itemsByExerciseID := make(map[string]*TodayExerciseHistory)
	items := make([]*TodayExerciseHistory, 0)
	for _, record := range records {
		item := itemsByExerciseID[record.ExerciseID]
		if item == nil {
			item = &TodayExerciseHistory{
				ExerciseID:           record.ExerciseID,
				ExerciseNameKey:      record.ExerciseNameKey,
				ExerciseNameSnapshot: record.ExerciseNameSnapshot,
				CategoryKey:          record.CategoryKey,
				TypeKey:              record.TypeKey,
				WeightUnit:           record.WeightUnit,
				LatestRecordedAt:     record.RecordedAt,
				LatestRecords:        make([]*ExerciseSetRecord, 0, TodayExerciseLatestLimit),
			}
			itemsByExerciseID[record.ExerciseID] = item
			items = append(items, item)
		}

		item.SetCount++
		if record.WeightX10 > item.MaxWeightX10 {
			item.MaxWeightX10 = record.WeightX10
			item.WeightUnit = record.WeightUnit
		}
		if len(item.LatestRecords) < TodayExerciseLatestLimit {
			item.LatestRecords = append(item.LatestRecords, record)
		}
	}

	sort.SliceStable(items, func(i, j int) bool {
		return items[i].LatestRecordedAt > items[j].LatestRecordedAt
	})

	return items, nil
}

// CountExerciseSetRecordChanges 统计快照范围内变更过的动作记录。
func CountExerciseSetRecordChanges(uid uint64, startSnapshotID int64, endSnapshotID int64) (uint64, string, string, error) {
	if uid == 0 {
		return 0, "", "", fmt.Errorf("uid is empty")
	}
	if endSnapshotID <= 0 || startSnapshotID > endSnapshotID {
		return 0, "", "", fmt.Errorf("snapshot range is invalid")
	}

	db, err := config.DB()
	if err != nil {
		return 0, "", "", err
	}

	var count int64
	if err := exerciseSetRecordChangesQuery(db, uid, startSnapshotID, endSnapshotID).
		Model(&ExerciseSetRecord{}).
		Count(&count).Error; err != nil {
		return 0, "", "", err
	}
	if count == 0 {
		return 0, "", "", nil
	}

	startDate, endDate, err := exerciseSetRecordDateRange(
		exerciseSetRecordChangesQuery(db, uid, startSnapshotID, endSnapshotID).Model(&ExerciseSetRecord{}),
	)
	if err != nil {
		return 0, "", "", err
	}
	return uint64(count), startDate, endDate, nil
}

// ListExerciseSetRecordChangesPage 分页查询快照范围内变更过的动作记录。
func ListExerciseSetRecordChangesPage(uid uint64, startSnapshotID int64, endSnapshotID int64, limit int, offset int) ([]*ExerciseSetRecord, error) {
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

	var records []*ExerciseSetRecord
	if err := exerciseSetRecordChangesQuery(db, uid, startSnapshotID, endSnapshotID).
		Order(exerciseSetRecordChangedAtSQL() + " ASC, recorded_at ASC, id ASC").
		Limit(limit).
		Offset(offset).
		Find(&records).Error; err != nil {
		return nil, err
	}
	return records, nil
}

func normalizeExerciseRecordPageSize(pageSize int32) int {
	if pageSize <= 0 {
		return DefaultExerciseSetPageSize
	}
	if pageSize > MaxExerciseSetRecordPageSize {
		return MaxExerciseSetRecordPageSize
	}
	return int(pageSize)
}

func parseExerciseRecordCursor(cursor string) (int64, uint64, error) {
	cursor = strings.TrimSpace(cursor)
	if cursor == "" {
		return 0, 0, nil
	}

	parts := strings.Split(cursor, ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("cursor is invalid")
	}
	recordedAt, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || recordedAt <= 0 {
		return 0, 0, fmt.Errorf("cursor recorded_at is invalid")
	}
	id, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil || id == 0 {
		return 0, 0, fmt.Errorf("cursor id is invalid")
	}
	return recordedAt, id, nil
}

func formatExerciseRecordCursor(recordedAt int64, id uint64) string {
	if recordedAt <= 0 || id == 0 {
		return ""
	}
	return fmt.Sprintf("%d:%d", recordedAt, id)
}

func exerciseRecordDayBounds(recordDate string) (int64, int64, error) {
	var t time.Time
	var err error
	if strings.TrimSpace(recordDate) == "" {
		t = time.Now()
	} else {
		t, err = time.ParseInLocation("2006-01-02", recordDate, time.Local)
		if err != nil {
			return 0, 0, err
		}
	}

	start := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	return start.UnixMilli(), start.AddDate(0, 0, 1).UnixMilli(), nil
}

func exerciseSetRecordChangesQuery(db *gorm.DB, uid uint64, startSnapshotID int64, endSnapshotID int64) *gorm.DB {
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

func exerciseSetRecordChangedAtSQL() string {
	return "GREATEST(updated_at, COALESCE(deleted_at, updated_at))"
}

func exerciseSetRecordDateRange(query *gorm.DB) (string, string, error) {
	var result struct {
		MinRecordedAt int64
		MaxRecordedAt int64
	}
	if err := query.
		Select("MIN(recorded_at) AS min_recorded_at, MAX(recorded_at) AS max_recorded_at").
		Scan(&result).Error; err != nil {
		return "", "", err
	}
	if result.MinRecordedAt <= 0 || result.MaxRecordedAt <= 0 {
		return "", "", nil
	}
	return time.UnixMilli(result.MinRecordedAt).Format("2006-01-02"),
		time.UnixMilli(result.MaxRecordedAt).Format("2006-01-02"),
		nil
}

// ExerciseRecordToPB 将 MySQL 动作记录转换为 pb 模型。
func ExerciseRecordToPB(record *ExerciseSetRecord) *pb.ExerciseSetRecord {
	if record == nil {
		return nil
	}
	return &pb.ExerciseSetRecord{
		Id:                   record.ID,
		Uid:                  record.UID,
		ExerciseId:           record.ExerciseID,
		ExerciseNameKey:      record.ExerciseNameKey,
		ExerciseNameSnapshot: record.ExerciseNameSnapshot,
		CategoryKey:          record.CategoryKey,
		TypeKey:              record.TypeKey,
		WeightX10:            record.WeightX10,
		WeightUnit:           pb.ExerciseWeightUnit(record.WeightUnit),
		Reps:                 record.Reps,
		RecordedAt:           record.RecordedAt,
		CreatedAt:            record.CreatedAt.UnixMilli(),
		UpdatedAt:            record.UpdatedAt.UnixMilli(),
	}
}

package mysqlmodel

import (
	"fmt"
	"sort"
	"time"

	"spider-server/mysql/config"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// TrainingTag 表示用户可选择的训练标签定义。
//
// 设计说明：
// 1. UID = 0 的记录表示系统默认标签，例如 胸、肩、背、腿、手臂、腹。
// 2. UID > 0 的记录表示用户自定义标签。
// 3. 同一用户下标签名称唯一，避免重复创建同名标签。
// 4. 删除标签使用 GORM 软删除，历史绑定里的 TagName 仍可用于展示。
type TrainingTag struct {
	ID        uint64         `gorm:"primaryKey;autoIncrement"`
	UID       uint64         `gorm:"not null;uniqueIndex:idx_uid_name"`
	Name      string         `gorm:"type:varchar(64);not null;uniqueIndex:idx_uid_name"`
	Type      int32          `gorm:"not null"`
	SortOrder int32          `gorm:"not null;index"`
	Enabled   bool           `gorm:"not null;default:true"`
	CreatedAt time.Time      `gorm:"not null"`
	UpdatedAt time.Time      `gorm:"not null"`
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

// WorkoutTagBinding 表示某次 HealthKit 训练和训练标签之间的绑定关系。
//
// 设计说明：
// 1. 优先使用 WorkoutUUID 定位某次训练。
// 2. 为了兼容旧数据或兜底匹配，同时保存 WorkoutStartAt / WorkoutEndAt / WorkoutType。
// 3. TagName 冗余保存，避免标签后续被删除或改名后历史训练无法展示。
// 4. 同一用户、同一 WorkoutKey、同一 tag 只允许绑定一次。
// 5. WorkoutKey 优先使用 WorkoutUUID；WorkoutUUID 为空时，使用 start/end/type 生成兜底 key。
type WorkoutTagBinding struct {
	ID             uint64         `gorm:"primaryKey;autoIncrement"`
	UID            uint64         `gorm:"not null;index;uniqueIndex:idx_uid_workout_key_tag"`
	WorkoutKey     string         `gorm:"type:varchar(256);not null;index;uniqueIndex:idx_uid_workout_key_tag"`
	WorkoutUUID    string         `gorm:"type:varchar(128);not null;index"`
	WorkoutStartAt int64          `gorm:"not null;index"`
	WorkoutEndAt   int64          `gorm:"not null;index"`
	WorkoutType    string         `gorm:"type:varchar(64);not null"`
	TagID          uint64         `gorm:"not null;index;uniqueIndex:idx_uid_workout_key_tag"`
	TagName        string         `gorm:"type:varchar(64);not null"`
	RecordDate     string         `gorm:"type:varchar(20);not null;index"`
	CreatedAt      time.Time      `gorm:"not null"`
	UpdatedAt      time.Time      `gorm:"not null"`
	DeletedAt      gorm.DeletedAt `gorm:"index"`
}

// TrainingTagSortItem 表示标签排序更新项。
type TrainingTagSortItem struct {
	ID        uint64
	SortOrder int32
}

// DailyWorkoutTags 表示某一天内某次训练及其绑定标签。
type DailyWorkoutTags struct {
	WorkoutUUID    string
	WorkoutStartAt int64
	WorkoutEndAt   int64
	WorkoutType    string
	Bindings       []*WorkoutTagBinding
}

// DailyTrainingTagSummary 表示某一天训练标签聚合摘要。
type DailyTrainingTagSummary struct {
	RecordDate string
	TagNames   []string
	TagIDs     []uint64
}

const (
	TrainingTagTypeSystem int32 = 1
	TrainingTagTypeCustom int32 = 2
)

// CreateTrainingTag 创建用户自定义训练标签。
func CreateTrainingTag(uid uint64, name string, sortOrder int32) (*TrainingTag, error) {
	if uid == 0 {
		return nil, fmt.Errorf("uid is empty")
	}
	if name == "" {
		return nil, fmt.Errorf("name is empty")
	}

	tag := &TrainingTag{
		UID:       uid,
		Name:      name,
		Type:      TrainingTagTypeCustom,
		SortOrder: sortOrder,
		Enabled:   true,
	}

	db, err := config.DB()
	if err != nil {
		return nil, err
	}
	if err := db.Create(tag).Error; err != nil {
		return nil, err
	}

	return tag, nil
}

// UpdateTrainingTag 修改用户自定义训练标签。
func UpdateTrainingTag(uid uint64, id uint64, name string, sortOrder int32, enabled bool) (*TrainingTag, error) {
	if uid == 0 {
		return nil, fmt.Errorf("uid is empty")
	}
	if id == 0 {
		return nil, fmt.Errorf("id is empty")
	}
	if name == "" {
		return nil, fmt.Errorf("name is empty")
	}

	db, err := config.DB()
	if err != nil {
		return nil, err
	}

	result := db.Model(&TrainingTag{}).
		Where("uid = ? AND id = ?", uid, id).
		Updates(map[string]any{
			"name":       name,
			"sort_order": sortOrder,
			"enabled":    enabled,
		})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	return GetTrainingTagByID(uid, id)
}

// DeleteTrainingTag 删除用户自定义训练标签。
func DeleteTrainingTag(uid uint64, id uint64) error {
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

	return db.Transaction(func(tx *gorm.DB) error {
		result := tx.Where("uid = ? AND id = ?", uid, id).Delete(&TrainingTag{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}

		if err := tx.Where("uid = ? AND tag_id = ?", uid, id).Delete(&WorkoutTagBinding{}).Error; err != nil {
			return err
		}

		return nil
	})
}

// GetTrainingTagByID 根据 id 查询标签。
//
// 用户可访问自己的自定义标签，也可访问系统默认标签。
func GetTrainingTagByID(uid uint64, id uint64) (*TrainingTag, error) {
	if id == 0 {
		return nil, fmt.Errorf("id is empty")
	}

	db, err := config.DB()
	if err != nil {
		return nil, err
	}

	tag := &TrainingTag{}
	if err := db.Where("id = ? AND (uid = ? OR uid = 0)", id, uid).First(tag).Error; err != nil {
		return nil, err
	}
	return tag, nil
}

// ListTrainingTags 获取当前用户可用标签列表。
//
// 返回系统默认标签 + 用户自定义标签。
func ListTrainingTags(uid uint64, onlyEnabled bool) ([]*TrainingTag, error) {
	if uid == 0 {
		return nil, fmt.Errorf("uid is empty")
	}

	db, err := config.DB()
	if err != nil {
		return nil, err
	}

	query := db.Where("uid = 0 OR uid = ?", uid)
	if onlyEnabled {
		query = query.Where("enabled = ?", true)
	}

	var tags []*TrainingTag
	if err := query.Order("sort_order ASC, id ASC").Find(&tags).Error; err != nil {
		return nil, err
	}
	return tags, nil
}

// ReorderTrainingTags 批量调整当前用户自定义标签排序。
func ReorderTrainingTags(uid uint64, items []TrainingTagSortItem) ([]*TrainingTag, error) {
	if uid == 0 {
		return nil, fmt.Errorf("uid is empty")
	}

	db, err := config.DB()
	if err != nil {
		return nil, err
	}

	err = db.Transaction(func(tx *gorm.DB) error {
		for _, item := range items {
			if item.ID == 0 {
				continue
			}
			if err := tx.Model(&TrainingTag{}).
				Where("uid = ? AND id = ?", uid, item.ID).
				Update("sort_order", item.SortOrder).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return ListTrainingTags(uid, false)
}

// SaveWorkoutTags 覆盖式保存某次训练绑定的完整标签列表。
//
// 保存规则：
// 1. 同一个 uid + workout_key + tag_id 已存在时更新绑定信息。
// 2. 如果旧绑定被软删除，本次保存会恢复 deleted_at。
// 3. 本次没有传入的旧 tag 绑定会被删除。
func SaveWorkoutTags(uid uint64, workoutUUID string, workoutStartAt int64, workoutEndAt int64, workoutType string, tagIDs []uint64) ([]*WorkoutTagBinding, error) {
	if uid == 0 {
		return nil, fmt.Errorf("uid is empty")
	}
	if workoutUUID == "" && (workoutStartAt == 0 || workoutEndAt == 0) {
		return nil, fmt.Errorf("workout identity is empty")
	}

	db, err := config.DB()
	if err != nil {
		return nil, err
	}

	recordDate := recordDateFromMillis(workoutStartAt)
	workoutKey := buildWorkoutKey(workoutUUID, workoutStartAt, workoutEndAt, workoutType)
	uniqueTagIDs := uniqueUint64s(tagIDs)

	err = db.Transaction(func(tx *gorm.DB) error {
		deleteQuery := tx.Where("uid = ? AND workout_key = ?", uid, workoutKey)
		if len(uniqueTagIDs) > 0 {
			deleteQuery = deleteQuery.Where("tag_id NOT IN ?", uniqueTagIDs)
		}
		if err := deleteQuery.Delete(&WorkoutTagBinding{}).Error; err != nil {
			return err
		}

		if len(uniqueTagIDs) == 0 {
			return nil
		}

		tags, err := listTrainingTagsByIDs(tx, uid, uniqueTagIDs)
		if err != nil {
			return err
		}
		tagMap := make(map[uint64]*TrainingTag, len(tags))
		for _, tag := range tags {
			tagMap[tag.ID] = tag
		}

		bindings := make([]*WorkoutTagBinding, 0, len(uniqueTagIDs))
		for _, tagID := range uniqueTagIDs {
			tag := tagMap[tagID]
			if tag == nil {
				return fmt.Errorf("tag not found: %d", tagID)
			}

			bindings = append(bindings, &WorkoutTagBinding{
				UID:            uid,
				WorkoutKey:     workoutKey,
				WorkoutUUID:    workoutUUID,
				WorkoutStartAt: workoutStartAt,
				WorkoutEndAt:   workoutEndAt,
				WorkoutType:    workoutType,
				TagID:          tag.ID,
				TagName:        tag.Name,
				RecordDate:     recordDate,
			})
		}

		return tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "uid"},
				{Name: "workout_key"},
				{Name: "tag_id"},
			},
			DoUpdates: clause.Assignments(map[string]any{
				"workout_uuid":     workoutUUID,
				"workout_start_at": workoutStartAt,
				"workout_end_at":   workoutEndAt,
				"workout_type":     workoutType,
				"tag_name":         gorm.Expr("VALUES(tag_name)"),
				"record_date":      recordDate,
				"deleted_at":       nil,
				"updated_at":       time.Now(),
			}),
		}).Create(&bindings).Error
	})
	if err != nil {
		return nil, err
	}

	return GetWorkoutTags(uid, workoutUUID, workoutStartAt, workoutEndAt)
}

// GetWorkoutTags 获取某次训练绑定的标签列表。
func GetWorkoutTags(uid uint64, workoutUUID string, workoutStartAt int64, workoutEndAt int64) ([]*WorkoutTagBinding, error) {
	if uid == 0 {
		return nil, fmt.Errorf("uid is empty")
	}
	if workoutUUID == "" && (workoutStartAt == 0 || workoutEndAt == 0) {
		return nil, fmt.Errorf("workout identity is empty")
	}

	workoutKey := buildWorkoutKey(workoutUUID, workoutStartAt, workoutEndAt, "")

	db, err := config.DB()
	if err != nil {
		return nil, err
	}

	query := db.Where("uid = ?", uid)
	if workoutUUID != "" {
		query = query.Where("workout_uuid = ?", workoutUUID)
	} else {
		query = query.Where("workout_key = ?", workoutKey)
	}

	var bindings []*WorkoutTagBinding
	if err := query.Order("id ASC").Find(&bindings).Error; err != nil {
		return nil, err
	}
	return bindings, nil
}

// DeleteWorkoutTags 删除某次训练绑定的全部标签。
func DeleteWorkoutTags(uid uint64, workoutUUID string, workoutStartAt int64, workoutEndAt int64) error {
	if uid == 0 {
		return fmt.Errorf("uid is empty")
	}
	if workoutUUID == "" && (workoutStartAt == 0 || workoutEndAt == 0) {
		return fmt.Errorf("workout identity is empty")
	}

	workoutKey := buildWorkoutKey(workoutUUID, workoutStartAt, workoutEndAt, "")

	db, err := config.DB()
	if err != nil {
		return err
	}

	query := db.Where("uid = ?", uid)
	if workoutUUID != "" {
		query = query.Where("workout_uuid = ?", workoutUUID)
	} else {
		query = query.Where("workout_key = ?", workoutKey)
	}

	return query.Delete(&WorkoutTagBinding{}).Error
}

// ListDailyWorkoutTags 获取某一天内所有训练及其标签。
func ListDailyWorkoutTags(uid uint64, recordDate string) ([]*DailyWorkoutTags, error) {
	if uid == 0 {
		return nil, fmt.Errorf("uid is empty")
	}
	if recordDate == "" {
		return nil, fmt.Errorf("record_date is empty")
	}

	db, err := config.DB()
	if err != nil {
		return nil, err
	}

	var bindings []*WorkoutTagBinding
	if err := db.Where("uid = ? AND record_date = ?", uid, recordDate).
		Order("workout_start_at ASC, id ASC").
		Find(&bindings).Error; err != nil {
		return nil, err
	}

	return groupBindingsByWorkout(bindings), nil
}

// ListRangeWorkoutTags 获取一段日期内按天聚合的训练标签摘要。
func ListRangeWorkoutTags(uid uint64, startDate string, endDate string) ([]*DailyTrainingTagSummary, error) {
	if uid == 0 {
		return nil, fmt.Errorf("uid is empty")
	}
	if startDate == "" || endDate == "" {
		return nil, fmt.Errorf("start_date and end_date are required")
	}

	db, err := config.DB()
	if err != nil {
		return nil, err
	}

	var bindings []*WorkoutTagBinding
	if err := db.Where("uid = ? AND record_date >= ? AND record_date <= ?", uid, startDate, endDate).
		Order("record_date ASC, workout_start_at ASC, id ASC").
		Find(&bindings).Error; err != nil {
		return nil, err
	}

	dayMap := make(map[string]*DailyTrainingTagSummary)
	for _, binding := range bindings {
		day := dayMap[binding.RecordDate]
		if day == nil {
			day = &DailyTrainingTagSummary{RecordDate: binding.RecordDate}
			dayMap[binding.RecordDate] = day
		}
		if !containsString(day.TagNames, binding.TagName) {
			day.TagNames = append(day.TagNames, binding.TagName)
		}
		if !containsUint64(day.TagIDs, binding.TagID) {
			day.TagIDs = append(day.TagIDs, binding.TagID)
		}
	}

	days := make([]*DailyTrainingTagSummary, 0, len(dayMap))
	for _, day := range dayMap {
		days = append(days, day)
	}
	sort.Slice(days, func(i, j int) bool {
		return days[i].RecordDate < days[j].RecordDate
	})
	return days, nil
}

func listTrainingTagsByIDs(tx *gorm.DB, uid uint64, tagIDs []uint64) ([]*TrainingTag, error) {
	ids := uniqueUint64s(tagIDs)
	if len(ids) == 0 {
		return nil, nil
	}

	var tags []*TrainingTag
	if err := tx.Where("id IN ? AND (uid = ? OR uid = 0) AND enabled = ?", ids, uid, true).Find(&tags).Error; err != nil {
		return nil, err
	}
	return tags, nil
}

func groupBindingsByWorkout(bindings []*WorkoutTagBinding) []*DailyWorkoutTags {
	workoutMap := make(map[string]*DailyWorkoutTags)
	order := make([]string, 0)

	for _, binding := range bindings {
		key := workoutBindingKey(binding)
		workout := workoutMap[key]
		if workout == nil {
			workout = &DailyWorkoutTags{
				WorkoutUUID:    binding.WorkoutUUID,
				WorkoutStartAt: binding.WorkoutStartAt,
				WorkoutEndAt:   binding.WorkoutEndAt,
				WorkoutType:    binding.WorkoutType,
			}
			workoutMap[key] = workout
			order = append(order, key)
		}
		workout.Bindings = append(workout.Bindings, binding)
	}

	result := make([]*DailyWorkoutTags, 0, len(order))
	for _, key := range order {
		result = append(result, workoutMap[key])
	}
	return result
}

func workoutBindingKey(binding *WorkoutTagBinding) string {
	if binding.WorkoutKey != "" {
		return binding.WorkoutKey
	}
	return buildWorkoutKey(binding.WorkoutUUID, binding.WorkoutStartAt, binding.WorkoutEndAt, binding.WorkoutType)
}

func buildWorkoutKey(workoutUUID string, workoutStartAt int64, workoutEndAt int64, workoutType string) string {
	if workoutUUID != "" {
		return "uuid:" + workoutUUID
	}
	return fmt.Sprintf("time:%d_%d_%s", workoutStartAt, workoutEndAt, workoutType)
}

func recordDateFromMillis(millis int64) string {
	if millis <= 0 {
		return ""
	}
	return time.UnixMilli(millis).Format("2006-01-02")
}

func uniqueUint64s(values []uint64) []uint64 {
	seen := make(map[uint64]bool)
	result := make([]uint64, 0, len(values))
	for _, value := range values {
		if value == 0 || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

func containsUint64(values []uint64, target uint64) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

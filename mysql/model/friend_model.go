package mysqlmodel

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"spider-server/mysql/config"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	FriendRequestStatusPending  int32 = 1
	FriendRequestStatusAccepted int32 = 2
	FriendRequestStatusRejected int32 = 3
)

// FriendProfileRecord 表示用户朋友资料。
type FriendProfileRecord struct {
	ID                  uint64         `gorm:"primaryKey;autoIncrement"`
	UID                 uint64         `gorm:"not null;uniqueIndex"`
	UserID              string         `gorm:"type:varchar(32);not null;uniqueIndex"`
	Nickname            string         `gorm:"type:varchar(64);not null;default:''"`
	AvatarSymbol        string         `gorm:"type:varchar(64);not null;default:'person.fill'"`
	Bio                 string         `gorm:"type:varchar(512);not null;default:''"`
	PlanTitle           string         `gorm:"type:varchar(128);not null;default:''"`
	PlanDescription     string         `gorm:"type:varchar(512);not null;default:''"`
	TrainingDataVisible bool           `gorm:"not null;default:true"`
	SparkDays           int32          `gorm:"not null;default:0"`
	RecentTrainingJSON  string         `gorm:"type:json"`
	SnapshotUpdatedAt   int64          `gorm:"not null;default:0"`
	CreatedAt           time.Time      `gorm:"not null"`
	UpdatedAt           time.Time      `gorm:"not null"`
	DeletedAt           gorm.DeletedAt `gorm:"index"`
}

// FriendRequestRecord 表示一条好友申请。
type FriendRequestRecord struct {
	ID        uint64         `gorm:"primaryKey;autoIncrement"`
	FromUID   uint64         `gorm:"not null;index:idx_friend_request_pair"`
	ToUID     uint64         `gorm:"not null;index:idx_friend_request_pair"`
	Message   string         `gorm:"type:varchar(256);not null;default:''"`
	Status    int32          `gorm:"not null;index"`
	HandledAt int64          `gorm:"not null;default:0"`
	CreatedAt time.Time      `gorm:"not null"`
	UpdatedAt time.Time      `gorm:"not null"`
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

// FriendRelationRecord 表示一条单向好友关系。
type FriendRelationRecord struct {
	ID        uint64         `gorm:"primaryKey;autoIncrement"`
	UID       uint64         `gorm:"not null;uniqueIndex:idx_uid_friend_uid"`
	FriendUID uint64         `gorm:"not null;uniqueIndex:idx_uid_friend_uid"`
	CreatedAt time.Time      `gorm:"not null"`
	UpdatedAt time.Time      `gorm:"not null"`
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

// FriendRemarkRecord 表示用户给好友设置的备注名。
type FriendRemarkRecord struct {
	ID        uint64         `gorm:"primaryKey;autoIncrement"`
	UID       uint64         `gorm:"not null;uniqueIndex:idx_remark_uid_friend_uid"`
	FriendUID uint64         `gorm:"not null;uniqueIndex:idx_remark_uid_friend_uid"`
	Remark    string         `gorm:"type:varchar(64);not null;default:''"`
	CreatedAt time.Time      `gorm:"not null"`
	UpdatedAt time.Time      `gorm:"not null"`
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

// FriendTrainingTagStatRecord 表示公开训练摘要中的标签热量。
type FriendTrainingTagStatRecord struct {
	Name     string `json:"name"`
	Calories string `json:"calories"`
}

// FriendTrainingDaySummaryRecord 表示公开训练摘要中的某一天。
type FriendTrainingDaySummaryRecord struct {
	RecordDate string                        `json:"record_date"`
	Tags       []FriendTrainingTagStatRecord `json:"tags"`
	Calories   string                        `json:"calories"`
}

func EnsureFriendProfile(uid uint64) (*FriendProfileRecord, error) {
	if uid == 0 {
		return nil, fmt.Errorf("uid is empty")
	}

	return ensureFriendProfile(uid, "")
}

func EnsureFriendProfileWithDefaultNickname(uid uint64, nickname string) (*FriendProfileRecord, error) {
	if uid == 0 {
		return nil, fmt.Errorf("uid is empty")
	}

	return ensureFriendProfile(uid, nickname)
}

func ensureFriendProfile(uid uint64, nickname string) (*FriendProfileRecord, error) {
	db, err := config.DB()
	if err != nil {
		return nil, err
	}

	profile := defaultFriendProfile(uid)
	if nickname = normalizeFriendNickname(nickname); nickname != "" {
		profile.Nickname = nickname
	}
	if err := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "uid"}},
		DoNothing: true,
	}).Create(profile).Error; err != nil {
		return nil, err
	}

	profile, err = GetFriendProfileByUID(uid)
	if err != nil {
		return nil, err
	}
	if nickname = normalizeFriendNickname(nickname); nickname != "" && profile.Nickname == defaultFriendNickname(uid) {
		if err := db.Model(&FriendProfileRecord{}).
			Where("uid = ? AND nickname = ?", uid, defaultFriendNickname(uid)).
			Update("nickname", nickname).Error; err != nil {
			return nil, err
		}
		return GetFriendProfileByUID(uid)
	}

	if !profile.TrainingDataVisible && profile.SnapshotUpdatedAt == 0 {
		if err := db.Model(&FriendProfileRecord{}).
			Where("uid = ? AND training_data_visible = ? AND snapshot_updated_at = 0", uid, false).
			Update("training_data_visible", true).Error; err != nil {
			return nil, err
		}
		return GetFriendProfileByUID(uid)
	}

	return profile, nil
}

func GetFriendProfileByUID(uid uint64) (*FriendProfileRecord, error) {
	if uid == 0 {
		return nil, fmt.Errorf("uid is empty")
	}

	db, err := config.DB()
	if err != nil {
		return nil, err
	}

	profile := &FriendProfileRecord{}
	if err := db.Where("uid = ?", uid).First(profile).Error; err != nil {
		return nil, err
	}
	return profile, nil
}

func GetFriendProfileByUserID(userID string) (*FriendProfileRecord, error) {
	if userID == "" {
		return nil, fmt.Errorf("user_id is empty")
	}

	db, err := config.DB()
	if err != nil {
		return nil, err
	}

	profile := &FriendProfileRecord{}
	if err := db.Where("user_id = ?", userID).First(profile).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if uid, ok := parseDefaultFriendUserID(userID); ok {
				if exists, existsErr := userExists(uid); existsErr != nil {
					return nil, existsErr
				} else if exists {
					return EnsureFriendProfile(uid)
				}
			}
		}
		return nil, err
	}
	return profile, nil
}

func UpdateFriendProfile(uid uint64, nickname string, avatarSymbol string, bio string, planTitle string, planDescription string) (*FriendProfileRecord, error) {
	if uid == 0 {
		return nil, fmt.Errorf("uid is empty")
	}

	if _, err := EnsureFriendProfile(uid); err != nil {
		return nil, err
	}
	if avatarSymbol == "" {
		avatarSymbol = "person.fill"
	}

	db, err := config.DB()
	if err != nil {
		return nil, err
	}

	if err := db.Model(&FriendProfileRecord{}).
		Where("uid = ?", uid).
		Updates(map[string]any{
			"nickname":         nickname,
			"avatar_symbol":    avatarSymbol,
			"bio":              bio,
			"plan_title":       planTitle,
			"plan_description": planDescription,
		}).Error; err != nil {
		return nil, err
	}

	return GetFriendProfileByUID(uid)
}

func ListFriendProfiles(uid uint64) ([]*FriendProfileRecord, map[uint64]int64, error) {
	if uid == 0 {
		return nil, nil, fmt.Errorf("uid is empty")
	}

	db, err := config.DB()
	if err != nil {
		return nil, nil, err
	}

	var relations []*FriendRelationRecord
	if err := db.Where("uid = ?", uid).Order("created_at ASC, id ASC").Find(&relations).Error; err != nil {
		return nil, nil, err
	}
	if len(relations) == 0 {
		return nil, map[uint64]int64{}, nil
	}

	friendUIDs := make([]uint64, 0, len(relations))
	createdAtMap := make(map[uint64]int64, len(relations))
	for _, relation := range relations {
		friendUIDs = append(friendUIDs, relation.FriendUID)
		createdAtMap[relation.FriendUID] = relation.CreatedAt.UnixMilli()
	}

	var profiles []*FriendProfileRecord
	if err := db.Where("uid IN ?", friendUIDs).Order("nickname ASC, uid ASC").Find(&profiles).Error; err != nil {
		return nil, nil, err
	}
	return profiles, createdAtMap, nil
}

func AddFriendRequest(fromUID uint64, friendUserID string) (string, error) {
	if fromUID == 0 {
		return "", fmt.Errorf("from_uid is empty")
	}
	if friendUserID == "" {
		return "", fmt.Errorf("friend_user_id is empty")
	}

	if _, err := EnsureFriendProfile(fromUID); err != nil {
		return "", err
	}
	toProfile, err := GetFriendProfileByUserID(friendUserID)
	if err != nil {
		return "", err
	}
	if toProfile.UID == fromUID {
		return "", fmt.Errorf("cannot add yourself")
	}

	db, err := config.DB()
	if err != nil {
		return "", err
	}

	exists, err := IsFriend(fromUID, toProfile.UID)
	if err != nil {
		return "", err
	}
	if exists {
		return "已经是好友", nil
	}

	var request FriendRequestRecord
	err = db.Where("from_uid = ? AND to_uid = ? AND status = ?", fromUID, toProfile.UID, FriendRequestStatusPending).
		First(&request).Error
	if err == nil {
		return "申请已发送", nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", err
	}

	request = FriendRequestRecord{
		FromUID: fromUID,
		ToUID:   toProfile.UID,
		Status:  FriendRequestStatusPending,
		Message: "请求添加你为好友",
	}
	if err := db.Create(&request).Error; err != nil {
		return "", err
	}
	return "申请已发送", nil
}

func ListReceivedFriendRequests(uid uint64) ([]*FriendRequestRecord, error) {
	if uid == 0 {
		return nil, fmt.Errorf("uid is empty")
	}

	db, err := config.DB()
	if err != nil {
		return nil, err
	}

	var requests []*FriendRequestRecord
	if err := db.Where("to_uid = ?", uid).
		Order("status ASC, created_at DESC, id DESC").
		Find(&requests).Error; err != nil {
		return nil, err
	}
	return requests, nil
}

func HandleFriendRequest(uid uint64, requestID string, accept bool) error {
	if uid == 0 {
		return fmt.Errorf("uid is empty")
	}
	requestUintID, err := strconv.ParseUint(requestID, 10, 64)
	if err != nil || requestUintID == 0 {
		return fmt.Errorf("request_id is invalid")
	}

	db, err := config.DB()
	if err != nil {
		return err
	}

	return db.Transaction(func(tx *gorm.DB) error {
		request := &FriendRequestRecord{}
		if err := tx.Where("id = ? AND to_uid = ?", requestUintID, uid).First(request).Error; err != nil {
			return err
		}
		if request.Status != FriendRequestStatusPending {
			return nil
		}

		status := FriendRequestStatusRejected
		if accept {
			status = FriendRequestStatusAccepted
		}
		if err := tx.Model(request).Updates(map[string]any{
			"status":     status,
			"handled_at": time.Now().UnixMilli(),
		}).Error; err != nil {
			return err
		}

		if !accept {
			return nil
		}

		if err := upsertFriendRelation(tx, request.FromUID, request.ToUID); err != nil {
			return err
		}
		return upsertFriendRelation(tx, request.ToUID, request.FromUID)
	})
}

func CountPendingFriendRequests(uid uint64) (int64, error) {
	if uid == 0 {
		return 0, fmt.Errorf("uid is empty")
	}

	db, err := config.DB()
	if err != nil {
		return 0, err
	}

	var count int64
	if err := db.Model(&FriendRequestRecord{}).
		Where("to_uid = ? AND status = ?", uid, FriendRequestStatusPending).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func IsFriend(uid uint64, friendUID uint64) (bool, error) {
	if uid == 0 || friendUID == 0 {
		return false, fmt.Errorf("uid is empty")
	}

	db, err := config.DB()
	if err != nil {
		return false, err
	}

	var count int64
	if err := db.Model(&FriendRelationRecord{}).
		Where("uid = ? AND friend_uid = ?", uid, friendUID).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func UpdateTrainingDataVisibility(uid uint64, visible bool, sparkDays int32, days []FriendTrainingDaySummaryRecord, updatedAt int64) (*FriendProfileRecord, error) {
	if uid == 0 {
		return nil, fmt.Errorf("uid is empty")
	}
	if _, err := EnsureFriendProfile(uid); err != nil {
		return nil, err
	}
	if updatedAt == 0 {
		updatedAt = time.Now().UnixMilli()
	}

	recentJSON, err := marshalFriendTrainingDays(days)
	if err != nil {
		return nil, err
	}

	db, err := config.DB()
	if err != nil {
		return nil, err
	}

	if err := db.Model(&FriendProfileRecord{}).
		Where("uid = ?", uid).
		Updates(map[string]any{
			"training_data_visible": visible,
			"spark_days":            sparkDays,
			"recent_training_json":  recentJSON,
			"snapshot_updated_at":   updatedAt,
		}).Error; err != nil {
		return nil, err
	}
	return GetFriendProfileByUID(uid)
}

func UploadTrainingPublicSnapshot(uid uint64, sparkDays int32, days []FriendTrainingDaySummaryRecord, updatedAt int64) error {
	profile, err := EnsureFriendProfile(uid)
	if err != nil {
		return err
	}
	if !profile.TrainingDataVisible {
		return fmt.Errorf("training data is not visible")
	}
	_, err = UpdateTrainingDataVisibility(uid, true, sparkDays, days, updatedAt)
	return err
}

func ParseFriendTrainingDays(raw string) []FriendTrainingDaySummaryRecord {
	if raw == "" {
		return nil
	}
	var days []FriendTrainingDaySummaryRecord
	if err := json.Unmarshal([]byte(raw), &days); err != nil {
		return nil
	}
	return days
}

func defaultFriendProfile(uid uint64) *FriendProfileRecord {
	return &FriendProfileRecord{
		UID:                 uid,
		UserID:              fmt.Sprintf("SP%06d", uid),
		Nickname:            defaultFriendNickname(uid),
		AvatarSymbol:        "person.fill",
		TrainingDataVisible: true,
		RecentTrainingJSON:  "[]",
	}
}

func defaultFriendNickname(uid uint64) string {
	return fmt.Sprintf("用户%d", uid)
}

func normalizeFriendNickname(nickname string) string {
	nickname = strings.TrimSpace(nickname)
	runes := []rune(nickname)
	if len(runes) <= 64 {
		return nickname
	}
	return string(runes[:64])
}

func parseDefaultFriendUserID(userID string) (uint64, bool) {
	if !strings.HasPrefix(userID, "SP") {
		return 0, false
	}
	uid, err := strconv.ParseUint(strings.TrimPrefix(userID, "SP"), 10, 64)
	if err != nil || uid == 0 {
		return 0, false
	}
	return uid, true
}

func userExists(uid uint64) (bool, error) {
	db, err := config.DB()
	if err != nil {
		return false, err
	}

	var count int64
	if err := db.Model(&User{}).Where("id = ?", uid).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func upsertFriendRelation(tx *gorm.DB, uid uint64, friendUID uint64) error {
	relation := &FriendRelationRecord{UID: uid, FriendUID: friendUID}
	return tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "uid"},
			{Name: "friend_uid"},
		},
		DoUpdates: clause.Assignments(map[string]any{
			"deleted_at": nil,
			"updated_at": time.Now(),
		}),
	}).Create(relation).Error
}

func marshalFriendTrainingDays(days []FriendTrainingDaySummaryRecord) (string, error) {
	if len(days) == 0 {
		return "[]", nil
	}
	data, err := json.Marshal(days)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// UpdateFriendRemark 更新好友备注名，remark 为空时清除备注。
func UpdateFriendRemark(uid uint64, friendUID uint64, remark string) error {
	if uid == 0 || friendUID == 0 {
		return fmt.Errorf("uid is empty")
	}

	isFriend, err := IsFriend(uid, friendUID)
	if err != nil {
		return err
	}
	if !isFriend {
		return gorm.ErrRecordNotFound
	}

	db, err := config.DB()
	if err != nil {
		return err
	}

	record := &FriendRemarkRecord{UID: uid, FriendUID: friendUID, Remark: remark}
	return db.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "uid"},
			{Name: "friend_uid"},
		},
		DoUpdates: clause.Assignments(map[string]any{
			"remark":     remark,
			"updated_at": time.Now(),
		}),
	}).Create(record).Error
}

// GetFriendRemarks 获取用户所有好友备注，返回 friendUID -> remark 映射。
func GetFriendRemarks(uid uint64) (map[uint64]string, error) {
	if uid == 0 {
		return nil, fmt.Errorf("uid is empty")
	}

	db, err := config.DB()
	if err != nil {
		return nil, err
	}

	var remarks []FriendRemarkRecord
	if err := db.Where("uid = ? AND remark != ''", uid).Find(&remarks).Error; err != nil {
		return nil, err
	}

	result := make(map[uint64]string, len(remarks))
	for _, r := range remarks {
		result[r.FriendUID] = r.Remark
	}
	return result, nil
}

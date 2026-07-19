package mysqlmodel

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"spider-server/mysql/config"

	drivermysql "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	FriendRequestStatusPending        int32 = 1
	FriendRequestStatusAccepted       int32 = 2
	FriendRequestStatusRejected       int32 = 3
	FriendPlanShareDispositionUsed    int32 = 1
	FriendPlanShareDispositionIgnored int32 = 2
	FriendSharedContentKindPlan       int32 = 1
	FriendSharedContentKindTraining   int32 = 2
	defaultFriendAvatarSymbol               = "profile_avatar_1"
	friendAvatarPrefix                      = "profile_avatar_"
	maxFriendAvatarIndex                    = 20
	maxPendingFriendPlanShares              = 20
)

var (
	ErrFriendPlanShareNotFriend     = errors.New("plan share recipient is not a friend")
	ErrFriendPlanSharePendingLimit  = errors.New("plan share pending limit reached")
	ErrFriendTrainingUseNotFriend   = errors.New("training use source is not a friend")
	ErrFriendTrainingUseUnavailable = errors.New("training use source is unavailable")
)

// FriendProfileRecord 表示用户朋友资料。
type FriendProfileRecord struct {
	ID                  uint64         `gorm:"primaryKey;autoIncrement"`
	UID                 uint64         `gorm:"not null;uniqueIndex"`
	UserID              string         `gorm:"type:varchar(32);not null;uniqueIndex"`
	Nickname            string         `gorm:"type:varchar(64);not null;default:''"`
	AvatarSymbol        string         `gorm:"type:varchar(64);not null;default:'profile_avatar_1'"`
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

// FriendPlanShareRecord 表示好友之间分享的一份不可变计划快照。
// 处理后保留 disposition/delete_reason，并通过 DeletedAt 软删除，不再参与同步。
type FriendPlanShareRecord struct {
	ID            uint64         `gorm:"primaryKey;autoIncrement"`
	FromUID       uint64         `gorm:"not null;index;uniqueIndex:idx_friend_plan_share_client"`
	ToUID         uint64         `gorm:"not null;index:idx_friend_plan_share_receiver"`
	ClientShareID string         `gorm:"type:varchar(64);not null;uniqueIndex:idx_friend_plan_share_client"`
	PlanJSON      string         `gorm:"type:json;not null"`
	Disposition   int32          `gorm:"not null;default:0"`
	DeleteReason  string         `gorm:"type:varchar(32);not null;default:''"`
	HandledAt     int64          `gorm:"not null;default:0"`
	CreatedAt     time.Time      `gorm:"not null"`
	UpdatedAt     time.Time      `gorm:"not null"`
	DeletedAt     gorm.DeletedAt `gorm:"index;index:idx_friend_plan_share_receiver"`
}

type FriendSharedPlanSetRecord struct {
	WeightText string `json:"weight_text"`
	RepsText   string `json:"reps_text"`
}

type FriendSharedPlanExerciseRecord struct {
	ExerciseID           string                      `json:"exercise_id"`
	NameKey              string                      `json:"name_key"`
	NameSnapshot         string                      `json:"name_snapshot"`
	CategoryKey          string                      `json:"category_key"`
	TypeKey              string                      `json:"type_key"`
	DisplayTypeKey       string                      `json:"display_type_key"`
	CustomName           string                      `json:"custom_name,omitempty"`
	CustomSubcategoryKey string                      `json:"custom_subcategory_key,omitempty"`
	CustomIntroduction   string                      `json:"custom_introduction,omitempty"`
	Note                 string                      `json:"note,omitempty"`
	SetCount             int32                       `json:"set_count"`
	WeightUnit           string                      `json:"weight_unit,omitempty"`
	Sets                 []FriendSharedPlanSetRecord `json:"sets"`
}

type FriendSharedPlanRecord struct {
	Title        string                           `json:"title"`
	SourcePlanID string                           `json:"source_plan_id,omitempty"`
	Exercises    []FriendSharedPlanExerciseRecord `json:"exercises"`
}

// FriendSharedContentScoreRecord records accumulated use points for an
// original shared plan or friend action-training session.
type FriendSharedContentScoreRecord struct {
	ID           uint64    `gorm:"primaryKey;autoIncrement"`
	ContentKind  int32     `gorm:"not null;uniqueIndex:idx_friend_shared_content_source,priority:1;index"`
	SourceUID    uint64    `gorm:"not null;uniqueIndex:idx_friend_shared_content_source,priority:2;index"`
	SourceID     string    `gorm:"type:varchar(256);not null;uniqueIndex:idx_friend_shared_content_source,priority:3"`
	Title        string    `gorm:"type:varchar(256);not null;default:''"`
	SnapshotJSON string    `gorm:"type:json;not null"`
	Score        int64     `gorm:"not null;default:0;index"`
	LastUsedAt   time.Time `gorm:"not null;index"`
	CreatedAt    time.Time `gorm:"not null"`
	UpdatedAt    time.Time `gorm:"not null"`
}

// FriendSharedContentUseEventRecord provides transport idempotency. The app
// creates a fresh client event for every confirmed use, so repeated uses by
// the same user still add points while a retry of one mutation does not.
type FriendSharedContentUseEventRecord struct {
	ID            uint64    `gorm:"primaryKey;autoIncrement"`
	ActorUID      uint64    `gorm:"not null;uniqueIndex:idx_friend_shared_content_use_event,priority:1"`
	ClientEventID string    `gorm:"type:varchar(64);not null;uniqueIndex:idx_friend_shared_content_use_event,priority:2"`
	ContentKind   int32     `gorm:"not null;index"`
	SourceUID     uint64    `gorm:"not null;index"`
	SourceID      string    `gorm:"type:varchar(256);not null"`
	CreatedAt     time.Time `gorm:"not null"`
}

// FriendTrainingTagStatRecord 表示公开训练摘要中的标签热量。
type FriendTrainingTagStatRecord struct {
	Name     string `json:"name"`
	Calories string `json:"calories"`
}

// FriendActionSetSummaryRecord 表示好友可见的一个动作组基础数据。
type FriendActionSetSummaryRecord struct {
	WeightX10  int32 `json:"weight_x10"`
	WeightUnit int32 `json:"weight_unit"`
	Reps       int32 `json:"reps"`
}

// FriendActionExerciseSummaryRecord 表示好友可见的一项动作及其有限组数。
type FriendActionExerciseSummaryRecord struct {
	ExerciseID           string                         `json:"exercise_id"`
	NameKey              string                         `json:"name_key"`
	NameSnapshot         string                         `json:"name_snapshot"`
	CategoryKey          string                         `json:"category_key"`
	TypeKey              string                         `json:"type_key"`
	CustomName           string                         `json:"custom_name,omitempty"`
	CustomSubcategoryKey string                         `json:"custom_subcategory_key,omitempty"`
	CustomIntroduction   string                         `json:"custom_introduction,omitempty"`
	Sets                 []FriendActionSetSummaryRecord `json:"sets"`
}

// FriendBoundWorkoutSummaryRecord 表示动作训练关联的 HealthKit 运动基础数据。
type FriendBoundWorkoutSummaryRecord struct {
	WorkoutType   string   `json:"workout_type"`
	StartAt       int64    `json:"start_at"`
	EndAt         int64    `json:"end_at"`
	DurationSecs  int32    `json:"duration_seconds"`
	EnergyKcal    float64  `json:"energy_kcal"`
	DistanceMeter *float64 `json:"distance_meters,omitempty"`
	Tags          []string `json:"tags,omitempty"`
}

// FriendActionTrainingSessionRecord 表示好友资料内的一次只读动作训练摘要。
type FriendActionTrainingSessionRecord struct {
	SessionID    string                              `json:"session_id"`
	StartAt      int64                               `json:"start_at"`
	EndAt        int64                               `json:"end_at"`
	Kind         int32                               `json:"kind"`
	Exercises    []FriendActionExerciseSummaryRecord `json:"exercises"`
	BoundWorkout *FriendBoundWorkoutSummaryRecord    `json:"bound_workout,omitempty"`
}

// FriendTrainingDaySummaryRecord 表示公开训练摘要中的某一天。
type FriendTrainingDaySummaryRecord struct {
	RecordDate             string                              `json:"record_date"`
	Tags                   []FriendTrainingTagStatRecord       `json:"tags"`
	Calories               string                              `json:"calories"`
	ActionTrainingSessions []FriendActionTrainingSessionRecord `json:"action_training_sessions,omitempty"`
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
	avatarSymbol = normalizeFriendAvatarSymbol(avatarSymbol)

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

func SendFriendPlanShare(fromUID uint64, toUID uint64, clientShareID string, plan FriendSharedPlanRecord) (*FriendPlanShareRecord, error) {
	if fromUID == 0 || toUID == 0 {
		return nil, fmt.Errorf("uid is empty")
	}
	clientShareID = strings.TrimSpace(clientShareID)
	if clientShareID == "" {
		return nil, fmt.Errorf("client_share_id is empty")
	}

	db, err := config.DB()
	if err != nil {
		return nil, err
	}

	existing := &FriendPlanShareRecord{}
	if err := db.Unscoped().Where("from_uid = ? AND client_share_id = ?", fromUID, clientShareID).First(existing).Error; err == nil {
		if existing.ToUID != toUID {
			return nil, fmt.Errorf("client_share_id recipient mismatch")
		}
		return existing, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	isFriend, err := IsFriend(fromUID, toUID)
	if err != nil {
		return nil, err
	}
	if !isFriend {
		return nil, ErrFriendPlanShareNotFriend
	}

	var pendingCount int64
	if err := db.Model(&FriendPlanShareRecord{}).Where("to_uid = ?", toUID).Count(&pendingCount).Error; err != nil {
		return nil, err
	}
	if pendingCount >= maxPendingFriendPlanShares {
		return nil, ErrFriendPlanSharePendingLimit
	}

	planJSON, err := json.Marshal(plan)
	if err != nil {
		return nil, err
	}
	record := &FriendPlanShareRecord{
		FromUID:       fromUID,
		ToUID:         toUID,
		ClientShareID: clientShareID,
		PlanJSON:      string(planJSON),
	}
	if err := db.Create(record).Error; err != nil {
		return nil, err
	}
	return record, nil
}

func ListReceivedFriendPlanShares(uid uint64) ([]*FriendPlanShareRecord, error) {
	if uid == 0 {
		return nil, fmt.Errorf("uid is empty")
	}
	db, err := config.DB()
	if err != nil {
		return nil, err
	}
	var records []*FriendPlanShareRecord
	if err := db.Where("to_uid = ?", uid).
		Order("created_at DESC, id DESC").
		Limit(maxPendingFriendPlanShares).
		Find(&records).Error; err != nil {
		return nil, err
	}
	return records, nil
}

func CountPendingFriendPlanShares(uid uint64) (int64, error) {
	if uid == 0 {
		return 0, fmt.Errorf("uid is empty")
	}
	db, err := config.DB()
	if err != nil {
		return 0, err
	}
	var count int64
	if err := db.Model(&FriendPlanShareRecord{}).Where("to_uid = ?", uid).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func HandleFriendPlanShare(uid uint64, shareID string, disposition int32) error {
	if uid == 0 {
		return fmt.Errorf("uid is empty")
	}
	shareUintID, err := strconv.ParseUint(shareID, 10, 64)
	if err != nil || shareUintID == 0 {
		return fmt.Errorf("share_id is invalid")
	}
	deleteReason, validDisposition := friendPlanShareDeleteReason(disposition)
	if !validDisposition {
		return fmt.Errorf("disposition is invalid")
	}

	db, err := config.DB()
	if err != nil {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		record := &FriendPlanShareRecord{}
		if err := tx.Unscoped().Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND to_uid = ?", shareUintID, uid).
			First(record).Error; err != nil {
			return err
		}
		if record.DeletedAt.Valid {
			return nil
		}
		now := time.Now()
		if disposition == FriendPlanShareDispositionUsed {
			plan, err := ParseFriendSharedPlan(record.PlanJSON)
			if err != nil {
				return err
			}
			sourceID := strings.TrimSpace(plan.SourcePlanID)
			if sourceID == "" {
				sourceID = "legacy-share:" + record.ClientShareID
			}
			if err := incrementFriendSharedContentScoreTx(
				tx,
				FriendSharedContentKindPlan,
				record.FromUID,
				sourceID,
				plan.Title,
				record.PlanJSON,
				now,
			); err != nil {
				return err
			}
		}
		return tx.Unscoped().Model(record).Updates(map[string]any{
			"disposition":   disposition,
			"delete_reason": deleteReason,
			"handled_at":    now.UnixMilli(),
			"deleted_at":    now,
		}).Error
	})
}

// RecordFriendTrainingUse adds one point after a friend action-training was
// actually copied into the actor's plan library.
func RecordFriendTrainingUse(actorUID uint64, clientEventID string, sourceUID uint64, trainingSessionID string) error {
	clientEventID = strings.TrimSpace(clientEventID)
	trainingSessionID = strings.TrimSpace(trainingSessionID)
	if actorUID == 0 || sourceUID == 0 {
		return fmt.Errorf("uid is empty")
	}
	if clientEventID == "" {
		return fmt.Errorf("client_event_id is empty")
	}
	if trainingSessionID == "" {
		return fmt.Errorf("training_session_id is empty")
	}

	isFriend, err := IsFriend(actorUID, sourceUID)
	if err != nil {
		return err
	}
	if !isFriend {
		return ErrFriendTrainingUseNotFriend
	}
	profile, err := GetFriendProfileByUID(sourceUID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrFriendTrainingUseUnavailable
		}
		return err
	}
	if !profile.TrainingDataVisible {
		return ErrFriendTrainingUseUnavailable
	}

	var sourceTraining *FriendActionTrainingSessionRecord
	for _, day := range ParseFriendTrainingDays(profile.RecentTrainingJSON) {
		for index := range day.ActionTrainingSessions {
			if day.ActionTrainingSessions[index].SessionID == trainingSessionID {
				training := day.ActionTrainingSessions[index]
				sourceTraining = &training
				break
			}
		}
		if sourceTraining != nil {
			break
		}
	}
	if sourceTraining == nil {
		return ErrFriendTrainingUseUnavailable
	}
	snapshot, err := json.Marshal(sourceTraining)
	if err != nil {
		return err
	}
	title := friendTrainingScoreTitle(*sourceTraining)

	db, err := config.DB()
	if err != nil {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		event := &FriendSharedContentUseEventRecord{
			ActorUID:      actorUID,
			ClientEventID: clientEventID,
			ContentKind:   FriendSharedContentKindTraining,
			SourceUID:     sourceUID,
			SourceID:      trainingSessionID,
		}
		if err := tx.Create(event).Error; err != nil {
			var mysqlErr *drivermysql.MySQLError
			if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
				return nil
			}
			return err
		}
		return incrementFriendSharedContentScoreTx(
			tx,
			FriendSharedContentKindTraining,
			sourceUID,
			trainingSessionID,
			title,
			string(snapshot),
			time.Now(),
		)
	})
}

func incrementFriendSharedContentScoreTx(tx *gorm.DB, kind int32, sourceUID uint64, sourceID string, title string, snapshotJSON string, usedAt time.Time) error {
	record := &FriendSharedContentScoreRecord{
		ContentKind:  kind,
		SourceUID:    sourceUID,
		SourceID:     sourceID,
		Title:        strings.TrimSpace(title),
		SnapshotJSON: snapshotJSON,
		Score:        1,
		LastUsedAt:   usedAt,
	}
	return tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "content_kind"},
			{Name: "source_uid"},
			{Name: "source_id"},
		},
		DoUpdates: clause.Assignments(map[string]any{
			"title":         record.Title,
			"snapshot_json": record.SnapshotJSON,
			"score":         gorm.Expr("score + ?", 1),
			"last_used_at":  usedAt,
			"updated_at":    usedAt,
		}),
	}).Create(record).Error
}

func friendTrainingScoreTitle(training FriendActionTrainingSessionRecord) string {
	if len(training.Exercises) == 0 {
		return "动作训练"
	}
	name := strings.TrimSpace(training.Exercises[0].CustomName)
	if name == "" {
		name = strings.TrimSpace(training.Exercises[0].NameSnapshot)
	}
	if name == "" {
		name = "动作训练"
	}
	if len(training.Exercises) == 1 {
		return name
	}
	return fmt.Sprintf("%s 等 %d 个动作", name, len(training.Exercises))
}

func friendPlanShareDeleteReason(disposition int32) (string, bool) {
	switch disposition {
	case FriendPlanShareDispositionUsed:
		return "used", true
	case FriendPlanShareDispositionIgnored:
		return "ignored", true
	default:
		return "", false
	}
}

func ParseFriendSharedPlan(raw string) (FriendSharedPlanRecord, error) {
	var plan FriendSharedPlanRecord
	if raw == "" {
		return plan, fmt.Errorf("plan_json is empty")
	}
	if err := json.Unmarshal([]byte(raw), &plan); err != nil {
		return FriendSharedPlanRecord{}, err
	}
	return plan, nil
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
		AvatarSymbol:        defaultFriendAvatarSymbol,
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

func normalizeFriendAvatarSymbol(avatarSymbol string) string {
	avatarSymbol = strings.TrimSpace(avatarSymbol)
	if avatarSymbol == "" {
		return defaultFriendAvatarSymbol
	}
	indexText := avatarSymbol
	if strings.HasPrefix(avatarSymbol, friendAvatarPrefix) {
		indexText = strings.TrimPrefix(avatarSymbol, friendAvatarPrefix)
	}
	index, err := strconv.Atoi(indexText)
	if err != nil || index < 1 || index > maxFriendAvatarIndex {
		return defaultFriendAvatarSymbol
	}
	return fmt.Sprintf("%s%d", friendAvatarPrefix, index)
}

func parseDefaultFriendUserID(userID string) (uint64, bool) {
	userID = strings.ToUpper(strings.TrimSpace(userID))
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

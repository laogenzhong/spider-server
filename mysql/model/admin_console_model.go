package mysqlmodel

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"spider-server/common/devicecatalog"
	"spider-server/mysql/config"

	"gorm.io/gorm"
)

const (
	AdminPaymentSourceAll       = "all"
	AdminPaymentSourcePurchase  = "purchase"
	AdminPaymentSourceOfferCode = "offer_code"

	AdminRefundStatusRequested = "requested"
	AdminRefundStatusCompleted = "completed"
)

type AdminPageQuery struct {
	Search   string
	From     *time.Time
	To       *time.Time
	Page     int
	PageSize int
}

type AdminUserDetail struct {
	UID                  uint64           `json:"uid"`
	Account              string           `json:"account"`
	Nickname             string           `json:"nickname"`
	AppleEmail           string           `json:"apple_email"`
	LastAppEnterAt       *time.Time       `json:"last_app_enter_at"`
	LastSystemLanguage   string           `json:"last_system_language"`
	LastAppVersion       string           `json:"last_app_version"`
	RegisterDeviceModel  string           `json:"register_device_model"`
	RegisterDeviceLabel  string           `json:"register_device_label"`
	RegisterIOSVersion   string           `json:"register_ios_version"`
	LastLoginDeviceModel string           `json:"last_login_device_model"`
	LastLoginDeviceLabel string           `json:"last_login_device_label"`
	LastLoginIOSVersion  string           `json:"last_login_ios_version"`
	LastLoginAt          *time.Time       `json:"last_login_at"`
	CreatedAt            time.Time        `json:"created_at"`
	UpdatedAt            time.Time        `json:"updated_at"`
	VIP                  CurrentVIPStatus `json:"vip"`
}

type AdminPaymentRecord struct {
	ID                    uint64     `json:"id"`
	UID                   uint64     `json:"uid"`
	Account               string     `json:"account"`
	Nickname              string     `json:"nickname"`
	TransactionID         string     `json:"transaction_id"`
	OriginalTransactionID string     `json:"original_transaction_id"`
	ProductID             string     `json:"product_id"`
	Environment           string     `json:"environment"`
	TransactionType       string     `json:"transaction_type" gorm:"column:transaction_type"`
	PurchaseAt            *time.Time `json:"purchase_at"`
	ExpiresAt             *time.Time `json:"expires_at"`
	RevocationAt          *time.Time `json:"revocation_at"`
	RevocationReason      int32      `json:"revocation_reason"`
	OfferType             int32      `json:"offer_type"`
	OfferIdentifier       string     `json:"offer_identifier"`
	Source                string     `json:"source" gorm:"-"`
	CreatedAt             time.Time  `json:"created_at"`
}

type AdminRefundRecord struct {
	ID                    uint64     `json:"id"`
	UID                   uint64     `json:"uid"`
	Account               string     `json:"account"`
	Nickname              string     `json:"nickname"`
	TransactionID         string     `json:"transaction_id"`
	OriginalTransactionID string     `json:"original_transaction_id"`
	ProductID             string     `json:"product_id"`
	Environment           string     `json:"environment"`
	NotificationUUID      string     `json:"notification_uuid"`
	NotificationType      string     `json:"notification_type"`
	ProcessingStatus      string     `json:"processing_status"`
	RequestedAt           *time.Time `json:"requested_at"`
	RevocationAt          *time.Time `json:"revocation_at"`
	RevocationReason      int32      `json:"revocation_reason"`
	OfferType             int32      `json:"offer_type"`
	OfferIdentifier       string     `json:"offer_identifier"`
	Source                string     `json:"source" gorm:"-"`
	Status                string     `json:"status" gorm:"-"`
}

type AdminUserListRecord struct {
	UID                 uint64     `json:"uid"`
	Account             string     `json:"account"`
	Nickname            string     `json:"nickname"`
	RegisterDeviceModel string     `json:"register_device_model"`
	RegisterDeviceLabel string     `json:"register_device_label" gorm:"-"`
	RegisterIOSVersion  string     `json:"register_ios_version"`
	LastLoginAt         *time.Time `json:"last_login_at"`
	LastAppEnterAt      *time.Time `json:"last_app_enter_at"`
	LastSystemLanguage  string     `json:"last_system_language"`
	CreatedAt           time.Time  `json:"created_at"`
	ActivityDate        *time.Time `json:"activity_date,omitempty"`
	FirstSeenAt         *time.Time `json:"first_seen_at,omitempty"`
	LastSeenAt          *time.Time `json:"last_seen_at,omitempty"`
	LastMethod          string     `json:"last_method,omitempty"`
	TouchCount          uint64     `json:"touch_count,omitempty"`
}

type AdminFeedbackRecord struct {
	ID        uint64    `json:"id"`
	UID       uint64    `json:"uid"`
	Account   string    `json:"account"`
	Nickname  string    `json:"nickname"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

type AdminOnboardingProfileRecord struct {
	ID            uint64    `json:"id"`
	UID           uint64    `json:"uid"`
	Account       string    `json:"account"`
	Nickname      string    `json:"nickname"`
	SchemaVersion int       `json:"schema_version"`
	CompletedAt   time.Time `json:"completed_at"`
	ProfileJSON   string    `json:"profile_json"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type AdminFriendProfileRecord struct {
	ID                  uint64    `json:"id"`
	UID                 uint64    `json:"uid"`
	Account             string    `json:"account"`
	UserID              string    `json:"user_id"`
	Nickname            string    `json:"nickname"`
	AvatarSymbol        string    `json:"avatar_symbol"`
	Bio                 string    `json:"bio"`
	PlanTitle           string    `json:"plan_title"`
	PlanDescription     string    `json:"plan_description"`
	TrainingDataVisible bool      `json:"training_data_visible"`
	SparkDays           int32     `json:"spark_days"`
	RecentTrainingJSON  string    `json:"recent_training_json"`
	SnapshotUpdatedAt   int64     `json:"snapshot_updated_at"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type AdminDailyFeatureRecord struct {
	Date             string `json:"date"`
	WeightUsers      int64  `json:"weight_users"`
	TrainingTagUsers int64  `json:"training_tag_users"`
	ExerciseSetUsers int64  `json:"exercise_set_users"`
	ExerciseSetCount int64  `json:"exercise_set_count"`
	CreatedPlanCount int64  `json:"created_plan_count"`
	UpdatedPlanCount int64  `json:"updated_plan_count"`
	BodyPhotoUsers   int64  `json:"body_photo_users"`
}

type adminDailyUIDCount struct {
	Date      string `gorm:"column:activity_date"`
	UserCount int64  `gorm:"column:user_count"`
}

type adminDailyRecordCount struct {
	Date        string `gorm:"column:activity_date"`
	RecordCount int64  `gorm:"column:record_count"`
}

func GetAdminUserDetail(identifier string, now time.Time) (*AdminUserDetail, error) {
	user, err := GetUserByAdminVIPIdentifier(identifier)
	if err != nil {
		return nil, err
	}
	status, err := GetCurrentVIPStatus(uint64(user.ID), now)
	if err != nil {
		return nil, err
	}
	detail := &AdminUserDetail{
		UID:                  uint64(user.ID),
		Account:              user.Account,
		LastAppEnterAt:       user.LastAppEnterAt,
		LastSystemLanguage:   user.LastSystemLanguage,
		LastAppVersion:       user.LastAppVersion,
		RegisterDeviceModel:  user.RegisterDeviceModel,
		RegisterDeviceLabel:  devicecatalog.DisplayLabel(user.RegisterDeviceModel),
		RegisterIOSVersion:   user.RegisterIOSVersion,
		LastLoginDeviceModel: user.LastLoginDeviceModel,
		LastLoginDeviceLabel: devicecatalog.DisplayLabel(user.LastLoginDeviceModel),
		LastLoginIOSVersion:  user.LastLoginIOSVersion,
		LastLoginAt:          user.LastLoginAt,
		CreatedAt:            user.CreatedAt,
		UpdatedAt:            user.UpdatedAt,
		VIP:                  status,
	}
	if profile, profileErr := GetFriendProfileByUID(uint64(user.ID)); profileErr == nil && profile != nil {
		detail.Nickname = profile.Nickname
	}
	if apple, appleErr := GetAppleSignInAccountByUserID(user.ID); appleErr == nil && apple != nil {
		detail.AppleEmail = strings.TrimSpace(apple.Email)
	}
	return detail, nil
}

func ListAdminPayments(query AdminPageQuery, source string) ([]AdminPaymentRecord, int64, error) {
	db, err := config.DB()
	if err != nil {
		return nil, 0, err
	}
	query = normalizeAdminPageQuery(query)
	source = normalizeAdminPaymentSource(source)
	base := db.Table("apple_transactions AS t").
		Joins("LEFT JOIN users AS u ON u.id = t.uid AND u.deleted_at IS NULL").
		Joins("LEFT JOIN friend_profile_records AS fp ON fp.uid = t.uid AND fp.deleted_at IS NULL").
		Where("t.deleted_at IS NULL")
	base = applyAdminPaymentSource(base, source, "t")
	base = applyAdminSearch(base, query.Search, "t", "u")
	base = applyAdminTimeRange(base, "t.purchase_at", query.From, query.To)

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	records := make([]AdminPaymentRecord, 0)
	err = base.Select(`
		t.id,
		t.uid,
		COALESCE(u.account, '') AS account,
		COALESCE(fp.nickname, '') AS nickname,
		t.transaction_id,
		t.original_transaction_id,
		t.product_id,
		t.environment,
		t.type AS transaction_type,
		t.purchase_at,
		t.expires_at,
		t.revocation_at,
		t.revocation_reason,
		t.offer_type,
		t.offer_identifier,
		t.created_at`).
		Order("t.purchase_at DESC, t.id DESC").
		Limit(query.PageSize).
		Offset((query.Page - 1) * query.PageSize).
		Scan(&records).Error
	if err != nil {
		return nil, 0, err
	}
	for i := range records {
		records[i].Source = adminPaymentSourceForOfferType(records[i].OfferType)
	}
	return records, total, nil
}

func ListAdminRefunds(query AdminPageQuery, status string, source string) ([]AdminRefundRecord, int64, error) {
	query = normalizeAdminPageQuery(query)
	status = strings.ToLower(strings.TrimSpace(status))
	if status == AdminRefundStatusCompleted {
		return listCompletedAdminRefunds(query, source)
	}
	return listRequestedAdminRefunds(query, source)
}

func listRequestedAdminRefunds(query AdminPageQuery, source string) ([]AdminRefundRecord, int64, error) {
	db, err := config.DB()
	if err != nil {
		return nil, 0, err
	}
	source = normalizeAdminPaymentSource(source)
	resolvedUID := "COALESCE(NULLIF(n.uid, 0), t.uid)"
	base := db.Table("app_store_server_notifications AS n").
		Joins(`JOIN apple_transactions AS t ON t.id = (
			SELECT t2.id
			FROM apple_transactions AS t2
			WHERE t2.deleted_at IS NULL AND (
				t2.transaction_id = n.transaction_id OR
				(n.original_transaction_id <> '' AND t2.original_transaction_id = n.original_transaction_id)
			)
			ORDER BY (t2.transaction_id = n.transaction_id) DESC, t2.id DESC
			LIMIT 1
		)`).
		Joins("LEFT JOIN users AS u ON u.id = "+resolvedUID+" AND u.deleted_at IS NULL").
		Joins("LEFT JOIN friend_profile_records AS fp ON fp.uid = "+resolvedUID+" AND fp.deleted_at IS NULL").
		Where("n.deleted_at IS NULL AND n.notification_type = ?", "CONSUMPTION_REQUEST")
	base = applyAdminPaymentSource(base, source, "t")
	base = applyAdminSearch(base, query.Search, "t", "u")
	base = applyAdminTimeRange(base, "n.notification_signed_at", query.From, query.To)

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	records := make([]AdminRefundRecord, 0)
	err = base.Select(`
		n.id,
		` + resolvedUID + ` AS uid,
		COALESCE(u.account, '') AS account,
		COALESCE(fp.nickname, '') AS nickname,
		n.transaction_id,
		n.original_transaction_id,
		n.product_id,
		n.environment,
		n.notification_uuid,
		n.notification_type,
		n.processing_status,
		n.notification_signed_at AS requested_at,
		n.revocation_at,
		n.revocation_reason,
		t.offer_type,
		t.offer_identifier`).
		Order("n.notification_signed_at DESC, n.id DESC").
		Limit(query.PageSize).
		Offset((query.Page - 1) * query.PageSize).
		Scan(&records).Error
	if err != nil {
		return nil, 0, err
	}
	decorateAdminRefunds(records, AdminRefundStatusRequested)
	return records, total, nil
}

func listCompletedAdminRefunds(query AdminPageQuery, source string) ([]AdminRefundRecord, int64, error) {
	db, err := config.DB()
	if err != nil {
		return nil, 0, err
	}
	source = normalizeAdminPaymentSource(source)
	base := db.Table("apple_transactions AS t").
		Joins("LEFT JOIN users AS u ON u.id = t.uid AND u.deleted_at IS NULL").
		Joins("LEFT JOIN friend_profile_records AS fp ON fp.uid = t.uid AND fp.deleted_at IS NULL").
		Where("t.deleted_at IS NULL AND t.revocation_at IS NOT NULL")
	base = applyAdminPaymentSource(base, source, "t")
	base = applyAdminSearch(base, query.Search, "t", "u")
	base = applyAdminTimeRange(base, "t.revocation_at", query.From, query.To)

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	records := make([]AdminRefundRecord, 0)
	err = base.Select(`
		t.id,
		t.uid,
		COALESCE(u.account, '') AS account,
		COALESCE(fp.nickname, '') AS nickname,
		t.transaction_id,
		t.original_transaction_id,
		t.product_id,
		t.environment,
		t.revocation_at,
		t.revocation_reason,
		t.offer_type,
		t.offer_identifier`).
		Order("t.revocation_at DESC, t.id DESC").
		Limit(query.PageSize).
		Offset((query.Page - 1) * query.PageSize).
		Scan(&records).Error
	if err != nil {
		return nil, 0, err
	}
	decorateAdminRefunds(records, AdminRefundStatusCompleted)
	return records, total, nil
}

func ListAdminDailyActivities(query AdminPageQuery) ([]AdminUserListRecord, int64, error) {
	query = normalizeAdminPageQuery(query)
	today := time.Now().In(time.Local)
	todayStart := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.Local)
	if query.To != nil && !query.To.After(todayStart) {
		return listAdminDailyActivitySnapshots(query)
	}
	return listAdminCurrentDailyActivities(query)
}

func listAdminCurrentDailyActivities(query AdminPageQuery) ([]AdminUserListRecord, int64, error) {
	db, err := config.DB()
	if err != nil {
		return nil, 0, err
	}
	base := db.Table("users AS u").
		Joins("LEFT JOIN friend_profile_records AS fp ON fp.uid = u.id AND fp.deleted_at IS NULL").
		Where("u.deleted_at IS NULL AND u.last_app_enter_at IS NOT NULL")
	base = applyAdminUserSearch(base, query.Search)
	base = applyAdminTimeRange(base, "u.last_app_enter_at", query.From, query.To)

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	records := make([]AdminUserListRecord, 0)
	err = base.Select(`
		u.id AS uid,
		u.account,
		COALESCE(fp.nickname, '') AS nickname,
		u.register_device_model,
		u.register_ios_version,
		u.last_login_at,
		u.last_app_enter_at,
		u.last_system_language,
		u.created_at,
		DATE(u.last_app_enter_at) AS activity_date,
		u.last_app_enter_at AS first_seen_at,
		u.last_app_enter_at AS last_seen_at`).
		Order("u.last_app_enter_at DESC, u.id DESC").
		Limit(query.PageSize).
		Offset((query.Page - 1) * query.PageSize).
		Scan(&records).Error
	if err != nil {
		return nil, 0, err
	}
	decorateAdminUsers(records)
	return records, total, nil
}

func listAdminDailyActivitySnapshots(query AdminPageQuery) ([]AdminUserListRecord, int64, error) {
	db, err := config.DB()
	if err != nil {
		return nil, 0, err
	}
	base := db.Table("daily_user_activity_snapshots AS a").
		Joins("JOIN users AS u ON u.id = a.uid AND u.deleted_at IS NULL").
		Joins("LEFT JOIN friend_profile_records AS fp ON fp.uid = a.uid AND fp.deleted_at IS NULL")
	base = applyAdminUserSearch(base, query.Search)
	base = applyAdminTimeRange(base, "a.activity_date", query.From, query.To)

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	records := make([]AdminUserListRecord, 0)
	err = base.Select(`
		a.uid,
		u.account,
		COALESCE(fp.nickname, '') AS nickname,
		u.register_device_model,
		u.register_ios_version,
		u.last_login_at,
		u.last_app_enter_at,
		u.last_system_language,
		u.created_at,
		a.activity_date,
		a.last_app_enter_at AS first_seen_at,
		a.last_app_enter_at AS last_seen_at`).
		Order("a.activity_date DESC, a.last_app_enter_at DESC, a.id DESC").
		Limit(query.PageSize).
		Offset((query.Page - 1) * query.PageSize).
		Scan(&records).Error
	if err != nil {
		return nil, 0, err
	}
	decorateAdminUsers(records)
	return records, total, nil
}

func ListAdminRegistrations(query AdminPageQuery) ([]AdminUserListRecord, int64, error) {
	db, err := config.DB()
	if err != nil {
		return nil, 0, err
	}
	query = normalizeAdminPageQuery(query)
	base := db.Table("users AS u").
		Joins("LEFT JOIN friend_profile_records AS fp ON fp.uid = u.id AND fp.deleted_at IS NULL").
		Where("u.deleted_at IS NULL")
	base = applyAdminUserSearch(base, query.Search)
	base = applyAdminTimeRange(base, "u.created_at", query.From, query.To)

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	records := make([]AdminUserListRecord, 0)
	err = base.Select(`
		u.id AS uid,
		u.account,
		COALESCE(fp.nickname, '') AS nickname,
		u.register_device_model,
		u.register_ios_version,
		u.last_login_at,
		u.last_app_enter_at,
		u.last_system_language,
		u.created_at`).
		Order("u.created_at DESC, u.id DESC").
		Limit(query.PageSize).
		Offset((query.Page - 1) * query.PageSize).
		Scan(&records).Error
	if err != nil {
		return nil, 0, err
	}
	decorateAdminUsers(records)
	return records, total, nil
}

func ListAdminFeedback(query AdminPageQuery) ([]AdminFeedbackRecord, int64, error) {
	db, err := config.DB()
	if err != nil {
		return nil, 0, err
	}
	query = normalizeAdminPageQuery(query)
	base := db.Table("user_feedbacks AS f").
		Joins("LEFT JOIN users AS u ON u.id = f.uid AND u.deleted_at IS NULL").
		Joins("LEFT JOIN friend_profile_records AS fp ON fp.uid = f.uid AND fp.deleted_at IS NULL").
		Where("f.deleted_at IS NULL")
	base = applyAdminFeedbackSearch(base, query.Search)
	base = applyAdminTimeRange(base, "f.created_at", query.From, query.To)

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	records := make([]AdminFeedbackRecord, 0)
	err = base.Select(`
		f.id,
		f.uid,
		COALESCE(u.account, '') AS account,
		COALESCE(fp.nickname, '') AS nickname,
		f.content,
		f.created_at`).
		Order("f.created_at DESC, f.id DESC").
		Limit(query.PageSize).
		Offset((query.Page - 1) * query.PageSize).
		Scan(&records).Error
	return records, total, err
}

func ListAdminOnboardingProfiles(query AdminPageQuery) ([]AdminOnboardingProfileRecord, int64, error) {
	db, err := config.DB()
	if err != nil {
		return nil, 0, err
	}
	query = normalizeAdminPageQuery(query)
	base := db.Table("onboarding_profiles AS o").
		Joins("LEFT JOIN users AS u ON u.id = o.uid AND u.deleted_at IS NULL").
		Joins("LEFT JOIN friend_profile_records AS fp ON fp.uid = o.uid AND fp.deleted_at IS NULL").
		Where("o.deleted_at IS NULL")
	base = applyAdminUserSearch(base, query.Search)
	base = applyAdminTimeRange(base, "o.completed_at", query.From, query.To)

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	records := make([]AdminOnboardingProfileRecord, 0)
	err = base.Select(`
		o.id,
		o.uid,
		COALESCE(u.account, '') AS account,
		COALESCE(fp.nickname, '') AS nickname,
		o.schema_version,
		o.completed_at,
		o.profile_json,
		o.created_at,
		o.updated_at`).
		Order("o.completed_at DESC, o.id DESC").
		Limit(query.PageSize).
		Offset((query.Page - 1) * query.PageSize).
		Scan(&records).Error
	return records, total, err
}

func ListAdminFriendProfiles(query AdminPageQuery) ([]AdminFriendProfileRecord, int64, error) {
	db, err := config.DB()
	if err != nil {
		return nil, 0, err
	}
	query = normalizeAdminPageQuery(query)
	base := db.Table("friend_profile_records AS fp").
		Joins("LEFT JOIN users AS u ON u.id = fp.uid AND u.deleted_at IS NULL").
		Where("fp.deleted_at IS NULL")
	base = applyAdminFriendProfileSearch(base, query.Search)
	base = applyAdminTimeRange(base, "fp.created_at", query.From, query.To)

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	records := make([]AdminFriendProfileRecord, 0)
	err = base.Select(`
		fp.id,
		fp.uid,
		COALESCE(u.account, '') AS account,
		fp.user_id,
		fp.nickname,
		fp.avatar_symbol,
		fp.bio,
		fp.plan_title,
		fp.plan_description,
		fp.training_data_visible,
		fp.spark_days,
		COALESCE(fp.recent_training_json, '[]') AS recent_training_json,
		fp.snapshot_updated_at,
		fp.created_at,
		fp.updated_at`).
		Order("fp.created_at DESC, fp.id DESC").
		Limit(query.PageSize).
		Offset((query.Page - 1) * query.PageSize).
		Scan(&records).Error
	return records, total, err
}

func ListAdminDailyFeatureAdoption(query AdminPageQuery) ([]AdminDailyFeatureRecord, int64, error) {
	db, err := config.DB()
	if err != nil {
		return nil, 0, err
	}
	query = normalizeAdminPageQuery(query)

	weight, err := listAdminDailyDistinctUIDs(db, "weight_records", "", query)
	if err != nil {
		return nil, 0, err
	}
	tags, err := listAdminDailyDistinctUIDs(db, "training_tags", "uid > 0", query)
	if err != nil {
		return nil, 0, err
	}
	exerciseSets, err := listAdminDailyDistinctUIDs(db, "exercise_set_records", "", query)
	if err != nil {
		return nil, 0, err
	}
	exerciseSetCounts, err := listAdminDailyRecordCounts(db, "exercise_set_records", "", "created_at", query)
	if err != nil {
		return nil, 0, err
	}
	createdPlans, err := listAdminDailyRecordCounts(
		db,
		"workout_data_snapshots",
		"kind = 4",
		"created_at",
		query,
	)
	if err != nil {
		return nil, 0, err
	}
	updatedPlans, err := listAdminDailyRecordCounts(
		db,
		"workout_data_snapshots",
		"kind = 4 AND deleted_at IS NULL AND updated_at > created_at",
		"updated_at",
		query,
	)
	if err != nil {
		return nil, 0, err
	}
	bodyPhotos, err := listAdminDailyDistinctUIDs(db, "body_photo_records", "", query)
	if err != nil {
		return nil, 0, err
	}

	return mergeAdminDailyFeatureRecords(query, weight, tags, exerciseSets, exerciseSetCounts, createdPlans, updatedPlans, bodyPhotos)
}

func listAdminDailyDistinctUIDs(db *gorm.DB, table string, condition string, query AdminPageQuery) ([]adminDailyUIDCount, error) {
	base := db.Table(table).
		Select("DATE(created_at) AS activity_date, COUNT(DISTINCT uid) AS user_count")
	if condition != "" {
		base = base.Where(condition)
	}
	base = applyAdminTimeRange(base, "created_at", query.From, query.To)
	records := make([]adminDailyUIDCount, 0)
	err := base.Group("DATE(created_at)").Scan(&records).Error
	return records, err
}

func listAdminDailyRecordCounts(db *gorm.DB, table string, condition string, timeColumn string, query AdminPageQuery) ([]adminDailyRecordCount, error) {
	base := db.Table(table).
		Select("DATE(" + timeColumn + ") AS activity_date, COUNT(*) AS record_count")
	if condition != "" {
		base = base.Where(condition)
	}
	base = applyAdminTimeRange(base, timeColumn, query.From, query.To)
	records := make([]adminDailyRecordCount, 0)
	err := base.Group("DATE(" + timeColumn + ")").Scan(&records).Error
	return records, err
}

func mergeAdminDailyFeatureRecords(
	query AdminPageQuery,
	weight []adminDailyUIDCount,
	tags []adminDailyUIDCount,
	exerciseSets []adminDailyUIDCount,
	exerciseSetCounts []adminDailyRecordCount,
	createdPlans []adminDailyRecordCount,
	updatedPlans []adminDailyRecordCount,
	bodyPhotos []adminDailyUIDCount,
) ([]AdminDailyFeatureRecord, int64, error) {
	byDate := make(map[string]*AdminDailyFeatureRecord)
	get := func(date string) *AdminDailyFeatureRecord {
		if byDate[date] == nil {
			byDate[date] = &AdminDailyFeatureRecord{Date: date}
		}
		return byDate[date]
	}
	for _, item := range weight {
		get(item.Date).WeightUsers = item.UserCount
	}
	for _, item := range tags {
		get(item.Date).TrainingTagUsers = item.UserCount
	}
	for _, item := range exerciseSets {
		get(item.Date).ExerciseSetUsers = item.UserCount
	}
	for _, item := range exerciseSetCounts {
		get(item.Date).ExerciseSetCount = item.RecordCount
	}
	for _, item := range createdPlans {
		get(item.Date).CreatedPlanCount = item.RecordCount
	}
	for _, item := range updatedPlans {
		get(item.Date).UpdatedPlanCount = item.RecordCount
	}
	for _, item := range bodyPhotos {
		get(item.Date).BodyPhotoUsers = item.UserCount
	}

	all := make([]AdminDailyFeatureRecord, 0, len(byDate))
	for _, item := range byDate {
		all = append(all, *item)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].Date > all[j].Date })
	total := int64(len(all))
	start := (query.Page - 1) * query.PageSize
	if start >= len(all) {
		return make([]AdminDailyFeatureRecord, 0), total, nil
	}
	end := start + query.PageSize
	if end > len(all) {
		end = len(all)
	}
	return all[start:end], total, nil
}

func normalizeAdminPageQuery(query AdminPageQuery) AdminPageQuery {
	query.Search = strings.TrimSpace(query.Search)
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.PageSize <= 0 {
		query.PageSize = 30
	}
	if query.PageSize > 100 {
		query.PageSize = 100
	}
	return query
}

func normalizeAdminPaymentSource(source string) string {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case AdminPaymentSourcePurchase:
		return AdminPaymentSourcePurchase
	case AdminPaymentSourceOfferCode:
		return AdminPaymentSourceOfferCode
	default:
		return AdminPaymentSourceAll
	}
}

func applyAdminPaymentSource(db *gorm.DB, source string, alias string) *gorm.DB {
	switch normalizeAdminPaymentSource(source) {
	case AdminPaymentSourceOfferCode:
		return db.Where(alias+".offer_type = ?", 3)
	case AdminPaymentSourcePurchase:
		return db.Where(alias+".offer_type <> ?", 3)
	default:
		return db
	}
}

func applyAdminSearch(db *gorm.DB, search string, transactionAlias string, userAlias string) *gorm.DB {
	search = strings.TrimSpace(search)
	if search == "" {
		return db
	}
	like := "%" + search + "%"
	if uid, err := strconv.ParseUint(search, 10, 64); err == nil && uid > 0 {
		return db.Where("("+transactionAlias+".uid = ? OR "+userAlias+".account LIKE ? OR "+transactionAlias+".transaction_id LIKE ? OR "+transactionAlias+".original_transaction_id LIKE ?)", uid, like, like, like)
	}
	return db.Where("("+userAlias+".account LIKE ? OR "+transactionAlias+".transaction_id LIKE ? OR "+transactionAlias+".original_transaction_id LIKE ?)", like, like, like)
}

func applyAdminUserSearch(db *gorm.DB, search string) *gorm.DB {
	search = strings.TrimSpace(search)
	if search == "" {
		return db
	}
	like := "%" + search + "%"
	if uid, err := strconv.ParseUint(search, 10, 64); err == nil && uid > 0 {
		return db.Where("(u.id = ? OR u.account LIKE ? OR fp.nickname LIKE ?)", uid, like, like)
	}
	return db.Where("(u.account LIKE ? OR fp.nickname LIKE ?)", like, like)
}

func applyAdminFeedbackSearch(db *gorm.DB, search string) *gorm.DB {
	search = strings.TrimSpace(search)
	if search == "" {
		return db
	}
	like := "%" + search + "%"
	if uid, err := strconv.ParseUint(search, 10, 64); err == nil && uid > 0 {
		return db.Where("(f.uid = ? OR u.account LIKE ? OR fp.nickname LIKE ? OR f.content LIKE ?)", uid, like, like, like)
	}
	return db.Where("(u.account LIKE ? OR fp.nickname LIKE ? OR f.content LIKE ?)", like, like, like)
}

func applyAdminFriendProfileSearch(db *gorm.DB, search string) *gorm.DB {
	search = strings.TrimSpace(search)
	if search == "" {
		return db
	}
	like := "%" + search + "%"
	if uid, err := strconv.ParseUint(search, 10, 64); err == nil && uid > 0 {
		return db.Where("(fp.uid = ? OR u.account LIKE ? OR fp.user_id LIKE ? OR fp.nickname LIKE ? OR fp.bio LIKE ?)", uid, like, like, like, like)
	}
	return db.Where("(u.account LIKE ? OR fp.user_id LIKE ? OR fp.nickname LIKE ? OR fp.bio LIKE ?)", like, like, like, like)
}

func applyAdminTimeRange(db *gorm.DB, column string, from *time.Time, to *time.Time) *gorm.DB {
	if from != nil {
		db = db.Where(column+" >= ?", *from)
	}
	if to != nil {
		db = db.Where(column+" < ?", *to)
	}
	return db
}

func adminPaymentSourceForOfferType(offerType int32) string {
	if offerType == 3 {
		return AdminPaymentSourceOfferCode
	}
	return AdminPaymentSourcePurchase
}

func decorateAdminRefunds(records []AdminRefundRecord, status string) {
	for i := range records {
		records[i].Source = adminPaymentSourceForOfferType(records[i].OfferType)
		records[i].Status = status
	}
}

func decorateAdminUsers(records []AdminUserListRecord) {
	for i := range records {
		records[i].RegisterDeviceLabel = devicecatalog.DisplayLabel(records[i].RegisterDeviceModel)
	}
}

func ValidateAdminDateRange(from *time.Time, to *time.Time) error {
	if from != nil && to != nil && !from.Before(*to) {
		return fmt.Errorf("start date must be before end date")
	}
	return nil
}

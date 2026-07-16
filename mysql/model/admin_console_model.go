package mysqlmodel

import (
	"fmt"
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

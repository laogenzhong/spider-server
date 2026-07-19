package mysqlmodel

import (
	"errors"
	"strings"
	"time"

	"spider-server/mysql/config"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ClientSyncFailure 保存客户端无法自动完成、需要人工排查或补偿的队列任务。
type ClientSyncFailure struct {
	ID                  uint64 `gorm:"primaryKey;autoIncrement"`
	UID                 uint64 `gorm:"not null;uniqueIndex:idx_client_sync_failure_uid_task,priority:1;index:idx_client_sync_failure_uid_created"`
	ClientTaskID        string `gorm:"type:varchar(64);not null;uniqueIndex:idx_client_sync_failure_uid_task,priority:2"`
	QueueName           string `gorm:"type:varchar(64);not null"`
	OriginalRPCPath     string `gorm:"type:varchar(255);not null"`
	OriginalRequestBody []byte `gorm:"type:longblob;not null"`
	RequestDataJSON     string `gorm:"type:longtext;not null"`
	BusinessCode        int32  `gorm:"not null;default:0"`
	BusinessMessage     string `gorm:"type:varchar(1000);not null"`
	AttemptCount        int32  `gorm:"not null;default:3"`
	ClientCreatedAt     int64  `gorm:"not null;default:0"`
	LastFailedAt        int64  `gorm:"not null;default:0;index:idx_client_sync_failure_status_failed,priority:2,sort:desc"`
	AppVersion          string `gorm:"type:varchar(64);not null"`
	Status              string `gorm:"type:varchar(20);not null;default:pending;index:idx_client_sync_failure_status_failed,priority:1"`
	ResolvedAt          *time.Time
	ResolvedBy          string    `gorm:"type:varchar(128);not null;default:''"`
	ResolutionNote      string    `gorm:"type:varchar(2000);not null;default:''"`
	CreatedAt           time.Time `gorm:"not null;index:idx_client_sync_failure_uid_created"`
	UpdatedAt           time.Time `gorm:"not null"`
}

var (
	ErrClientSyncFailureTaskIDEmpty  = errors.New("client sync failure task id empty")
	ErrClientSyncFailurePathEmpty    = errors.New("client sync failure rpc path empty")
	ErrAdminSyncFailureIDInvalid     = errors.New("admin sync failure id invalid")
	ErrAdminSyncFailureOperatorEmpty = errors.New("admin sync failure operator empty")
)

// ArchiveClientSyncFailure 幂等保存同一用户、同一客户端任务的最后失败详情。
func ArchiveClientSyncFailure(record *ClientSyncFailure) error {
	if record == nil || record.UID == 0 || strings.TrimSpace(record.ClientTaskID) == "" {
		return ErrClientSyncFailureTaskIDEmpty
	}
	if strings.TrimSpace(record.OriginalRPCPath) == "" {
		return ErrClientSyncFailurePathEmpty
	}

	db, err := config.DB()
	if err != nil {
		return err
	}

	return db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "uid"}, {Name: "client_task_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"queue_name",
			"original_rpc_path",
			"original_request_body",
			"request_data_json",
			"business_code",
			"business_message",
			"attempt_count",
			"client_created_at",
			"last_failed_at",
			"app_version",
			"status",
			"resolved_at",
			"resolved_by",
			"resolution_note",
			"updated_at",
		}),
	}).Create(record).Error
}

// ResolveAdminClientSyncFailure 只改变人工处理状态，不会自动重放原请求。
func ResolveAdminClientSyncFailure(id uint64, operator string, note string, resolvedAt time.Time) error {
	if id == 0 {
		return ErrAdminSyncFailureIDInvalid
	}
	operator = strings.TrimSpace(operator)
	if operator == "" {
		return ErrAdminSyncFailureOperatorEmpty
	}

	db, err := config.DB()
	if err != nil {
		return err
	}
	result := db.Model(&ClientSyncFailure{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":          AdminClientSyncFailureStatusResolved,
			"resolved_at":     resolvedAt,
			"resolved_by":     operator,
			"resolution_note": strings.TrimSpace(note),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

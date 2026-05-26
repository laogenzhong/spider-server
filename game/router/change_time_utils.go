package router

import (
	"time"

	"gorm.io/gorm"
)

func isDeletedInSnapshot(deletedAt gorm.DeletedAt, endSnapshotID int64) bool {
	return deletedAt.Valid && deletedAt.Time.UnixMilli() <= endSnapshotID
}

func deletedAtMillis(deletedAt gorm.DeletedAt, endSnapshotID int64) int64 {
	if !isDeletedInSnapshot(deletedAt, endSnapshotID) {
		return 0
	}
	return deletedAt.Time.UnixMilli()
}

func changedAtMillis(updatedAt time.Time, deletedAt gorm.DeletedAt, endSnapshotID int64) int64 {
	changedAt := updatedAt.UnixMilli()
	if isDeletedInSnapshot(deletedAt, endSnapshotID) && deletedAt.Time.UnixMilli() > changedAt {
		return deletedAt.Time.UnixMilli()
	}
	return changedAt
}

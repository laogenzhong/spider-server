package session

import (
	"context"
	"encoding/json"
	"errors"
	mysqlmodel "spider-server/common/mysql/model"
	"strconv"
	"time"

	"gorm.io/gorm"
)

type MySQLSessionStore struct{}

func NewMySQLSessionStore() *MySQLSessionStore {
	return &MySQLSessionStore{}
}

func (s *MySQLSessionStore) New(ctx context.Context, uid uint64, scopeID uint64, ttl time.Duration, attach map[string]string) error {
	copiedAttach := cloneStringMap(attach)

	if ttl > 0 {
		copiedAttach[attachTTLKey] = strconv.FormatInt(int64(ttl.Seconds()), 10)
	}

	attachText, err := marshalSessionAttach(copiedAttach)
	if err != nil {
		return err
	}

	expiresAt := buildSessionExpiresAt(ttl)

	_, err = mysqlmodel.CreateOrUpdateUserSession(uid, scopeID, attachText, expiresAt)
	return err
}

func (s *MySQLSessionStore) Put(ctx context.Context, uid uint64, scopeID uint64, ttl time.Duration, set map[string]string, remove []string) error {
	userSession, err := mysqlmodel.GetUserSession(uid, scopeID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrSessionNotFound
	}
	if err != nil {
		return err
	}

	if isUserSessionExpired(userSession) {
		_ = mysqlmodel.DeleteUserSession(uid, scopeID)
		return ErrSessionNotFound
	}

	attach, err := unmarshalSessionAttach(userSession.Attach)
	if err != nil {
		return err
	}

	for key, value := range set {
		attach[key] = value
	}

	for _, key := range remove {
		delete(attach, key)
	}

	expiresAt := userSession.ExpiresAt
	if ttl > 0 {
		attach[attachTTLKey] = strconv.FormatInt(int64(ttl.Seconds()), 10)
		expiresAt = buildSessionExpiresAt(ttl)
	}

	attachText, err := marshalSessionAttach(attach)
	if err != nil {
		return err
	}

	return mysqlmodel.UpdateUserSession(uid, scopeID, attachText, expiresAt)
}

func (s *MySQLSessionStore) Get(ctx context.Context, uid uint64, scopeID uint64, keys []string) (map[string]string, error) {
	userSession, err := mysqlmodel.GetUserSession(uid, scopeID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrSessionNotFound
	}
	if err != nil {
		return nil, err
	}

	if isUserSessionExpired(userSession) {
		_ = mysqlmodel.DeleteUserSession(uid, scopeID)
		return nil, ErrSessionNotFound
	}

	attach, err := unmarshalSessionAttach(userSession.Attach)
	if err != nil {
		return nil, err
	}

	if len(keys) == 0 {
		return cloneStringMap(attach), nil
	}

	result := make(map[string]string)
	for _, key := range keys {
		if value, ok := attach[key]; ok {
			result[key] = value
		}
	}

	return result, nil
}

func (s *MySQLSessionStore) Delete(ctx context.Context, uid uint64, scopeID uint64) error {
	return mysqlmodel.DeleteUserSession(uid, scopeID)
}

func marshalSessionAttach(attach map[string]string) (string, error) {
	if attach == nil {
		attach = make(map[string]string)
	}

	data, err := json.Marshal(attach)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

func unmarshalSessionAttach(attachText string) (map[string]string, error) {
	attach := make(map[string]string)
	if attachText == "" {
		return attach, nil
	}

	if err := json.Unmarshal([]byte(attachText), &attach); err != nil {
		return nil, err
	}

	return attach, nil
}

func buildSessionExpiresAt(ttl time.Duration) *time.Time {
	if ttl <= 0 {
		return nil
	}

	expiresAt := time.Now().Add(ttl)
	return &expiresAt
}

func isUserSessionExpired(userSession *mysqlmodel.UserSession) bool {
	if userSession == nil || userSession.ExpiresAt == nil {
		return false
	}

	return time.Now().After(*userSession.ExpiresAt)
}

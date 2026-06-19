package mysqlmodel

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"spider-server/mysql/config"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type AppleSignInAccount struct {
	ID             uint   `gorm:"primaryKey;autoIncrement"`
	UserID         uint   `gorm:"uniqueIndex;not null"`
	AppleSub       string `gorm:"size:255;uniqueIndex;not null"`
	Email          string `gorm:"size:255"`
	EmailVerified  bool
	IsPrivateEmail bool
	FullName       string `gorm:"size:255"`
	RefreshToken   string `gorm:"type:text"`
	AccessToken    string `gorm:"type:text"`
	AppleIDToken   string `gorm:"type:text"`
	TokenExpiresAt *time.Time
	LastLoginAt    time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      gorm.DeletedAt `gorm:"index"`
}

type AppleSignInAccountDeletionLog struct {
	ID                   uint   `gorm:"primaryKey;autoIncrement"`
	AppleSignInAccountID uint   `gorm:"index;not null"`
	UserID               uint   `gorm:"index;not null"`
	AppleSub             string `gorm:"size:255;index;not null"`
	Email                string `gorm:"size:255"`
	EmailVerified        bool
	IsPrivateEmail       bool
	FullName             string `gorm:"size:255"`
	RefreshToken         string `gorm:"type:text"`
	AccessToken          string `gorm:"type:text"`
	AppleIDToken         string `gorm:"type:text"`
	TokenExpiresAt       *time.Time
	LastLoginAt          time.Time
	OriginalCreatedAt    time.Time
	OriginalUpdatedAt    time.Time
	OriginalDeletedAt    *time.Time
	RevokeAppleSignIn    bool
	DeleteReason         string `gorm:"size:255"`
	DeletedAccountAt     time.Time
	CreatedAt            time.Time
}

type AppleSignInProfile struct {
	AppleSub       string
	Email          string
	EmailVerified  bool
	IsPrivateEmail bool
	FullName       string
	DeviceModel    string
	IOSVersion     string
	RefreshToken   string
	AccessToken    string
	IDToken        string
	ExpiresIn      int64
}

func GetAppleSignInAccountBySub(appleSub string) (*AppleSignInAccount, error) {
	appleSub = strings.TrimSpace(appleSub)
	if appleSub == "" {
		return nil, fmt.Errorf("apple sub is empty")
	}

	account := &AppleSignInAccount{}
	if err := config.First(account, "apple_sub = ?", appleSub); err != nil {
		return nil, err
	}

	return account, nil
}

func GetAppleSignInAccountByUserID(userID uint) (*AppleSignInAccount, error) {
	if userID == 0 {
		return nil, fmt.Errorf("user id is empty")
	}

	account := &AppleSignInAccount{}
	if err := config.First(account, "user_id = ?", userID); err != nil {
		return nil, err
	}

	return account, nil
}

func DeleteAppleSignInAccountByUserID(userID uint) error {
	if userID == 0 {
		return fmt.Errorf("user id is empty")
	}

	db, err := config.DB()
	if err != nil {
		return err
	}

	return db.Unscoped().Where("user_id = ?", userID).Delete(&AppleSignInAccount{}).Error
}

func ArchiveAndDeleteAppleSignInAccount(account *AppleSignInAccount, deletedAccountAt time.Time, revokeAppleSignIn bool, reason string) error {
	if account == nil || account.ID == 0 {
		return fmt.Errorf("apple sign in account is empty")
	}
	if deletedAccountAt.IsZero() {
		deletedAccountAt = time.Now()
	}

	var originalDeletedAt *time.Time
	if account.DeletedAt.Valid {
		deletedAt := account.DeletedAt.Time
		originalDeletedAt = &deletedAt
	}

	log := &AppleSignInAccountDeletionLog{
		AppleSignInAccountID: account.ID,
		UserID:               account.UserID,
		AppleSub:             account.AppleSub,
		Email:                account.Email,
		EmailVerified:        account.EmailVerified,
		IsPrivateEmail:       account.IsPrivateEmail,
		FullName:             account.FullName,
		RefreshToken:         account.RefreshToken,
		AccessToken:          account.AccessToken,
		AppleIDToken:         account.AppleIDToken,
		TokenExpiresAt:       account.TokenExpiresAt,
		LastLoginAt:          account.LastLoginAt,
		OriginalCreatedAt:    account.CreatedAt,
		OriginalUpdatedAt:    account.UpdatedAt,
		OriginalDeletedAt:    originalDeletedAt,
		RevokeAppleSignIn:    revokeAppleSignIn,
		DeleteReason:         strings.TrimSpace(reason),
		DeletedAccountAt:     deletedAccountAt,
	}

	return config.WithTx(func(tx *gorm.DB) error {
		if err := tx.Create(log).Error; err != nil {
			return err
		}

		result := tx.Unscoped().Where("id = ?", account.ID).Delete(&AppleSignInAccount{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}

		return nil
	})
}

func FindOrCreateUserForAppleSignIn(profile AppleSignInProfile, generatedAccount string) (*User, error) {
	if strings.TrimSpace(profile.AppleSub) == "" {
		return nil, fmt.Errorf("apple sub is empty")
	}
	if strings.TrimSpace(generatedAccount) == "" {
		return nil, fmt.Errorf("generated account is empty")
	}

	var user *User
	err := config.WithTx(func(tx *gorm.DB) error {
		binding := &AppleSignInAccount{}
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("apple_sub = ?", profile.AppleSub).
			First(binding).Error
		if err == nil {
			found := &User{}
			if err := tx.First(found, "id = ?", binding.UserID).Error; err != nil {
				return err
			}
			applyAppleProfile(binding, profile)
			if err := tx.Save(binding).Error; err != nil {
				return err
			}
			user = found
			return nil
		}
		if !isRecordNotFound(err) {
			return err
		}

		newUser := &User{
			Account:             generatedAccount,
			Password:            randomApplePassword(profile.AppleSub),
			RegisterDeviceModel: trimDeviceField(profile.DeviceModel, 64),
			RegisterIOSVersion:  trimDeviceField(profile.IOSVersion, 32),
		}
		if err := tx.Create(newUser).Error; err != nil {
			return err
		}

		binding = &AppleSignInAccount{
			UserID:   newUser.ID,
			AppleSub: profile.AppleSub,
		}
		applyAppleProfile(binding, profile)
		if err := tx.Create(binding).Error; err != nil {
			return err
		}

		user = newUser
		return nil
	})
	if err != nil {
		return nil, err
	}

	return user, nil
}

func applyAppleProfile(account *AppleSignInAccount, profile AppleSignInProfile) {
	if email := strings.TrimSpace(profile.Email); email != "" {
		account.Email = email
	}
	account.EmailVerified = profile.EmailVerified
	account.IsPrivateEmail = profile.IsPrivateEmail
	if fullName := strings.TrimSpace(profile.FullName); fullName != "" {
		account.FullName = fullName
	}
	if refreshToken := strings.TrimSpace(profile.RefreshToken); refreshToken != "" {
		account.RefreshToken = refreshToken
	}
	if accessToken := strings.TrimSpace(profile.AccessToken); accessToken != "" {
		account.AccessToken = accessToken
	}
	if idToken := strings.TrimSpace(profile.IDToken); idToken != "" {
		account.AppleIDToken = idToken
	}
	if profile.ExpiresIn > 0 {
		expiresAt := time.Now().Add(time.Duration(profile.ExpiresIn) * time.Second)
		account.TokenExpiresAt = &expiresAt
	}
	account.LastLoginAt = time.Now()
}

func isRecordNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}

func randomApplePassword(appleSub string) string {
	return "apple:" + stableHex(appleSub, 64)
}

func AppleGeneratedAccount(appleSub string) string {
	return "apple:" + stableHex(appleSub, 58)
}

func stableHex(value string, maxLen int) string {
	sum := sha256.Sum256([]byte(value))
	encoded := hex.EncodeToString(sum[:])
	if maxLen > 0 && len(encoded) > maxLen {
		return encoded[:maxLen]
	}
	return encoded
}

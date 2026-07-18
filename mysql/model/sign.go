package mysqlmodel

import (
	"fmt"
	"spider-server/mysql/config"
	"strings"
	"time"

	"gorm.io/gorm"
)

type User struct {
	ID                   uint   `gorm:"primaryKey;autoIncrement"`
	Account              string `gorm:"size:64;uniqueIndex;not null"`
	Password             string `gorm:"size:255;not null"`
	LastAppEnterAt       *time.Time
	LastSystemLanguage   string `gorm:"size:64"`
	LastAppVersion       string `gorm:"size:32"`
	RegisterDeviceModel  string `gorm:"size:64"`
	RegisterIOSVersion   string `gorm:"size:32"`
	LastLoginDeviceModel string `gorm:"size:64"`
	LastLoginIOSVersion  string `gorm:"size:32"`
	LastLoginAt          *time.Time
	CreatedAt            time.Time
	UpdatedAt            time.Time
	DeletedAt            gorm.DeletedAt `gorm:"index"`
}

func InitUserTable() error {
	return config.AutoMigrate(&User{})
}

func CreateUser(account string, password string) (*User, error) {
	if account == "" {
		return nil, fmt.Errorf("account is empty")
	}
	if password == "" {
		return nil, fmt.Errorf("password is empty")
	}

	user := &User{
		Account:  account,
		Password: password,
	}

	if err := config.Create(user); err != nil {
		return nil, err
	}

	return user, nil
}

func CreateUserWithRegistrationDevice(account string, password string, deviceModel string, iosVersion string) (*User, error) {
	if account == "" {
		return nil, fmt.Errorf("account is empty")
	}
	if password == "" {
		return nil, fmt.Errorf("password is empty")
	}

	user := &User{
		Account:             account,
		Password:            password,
		RegisterDeviceModel: trimDeviceField(deviceModel, 64),
		RegisterIOSVersion:  trimDeviceField(iosVersion, 32),
	}

	if err := config.Create(user); err != nil {
		return nil, err
	}

	return user, nil
}

func GetUserByAccount(account string) (*User, error) {
	if account == "" {
		return nil, fmt.Errorf("account is empty")
	}

	user := &User{}
	if err := config.First(user, "account = ?", account); err != nil {
		return nil, err
	}

	return user, nil
}

func UpdateUserPasswordByID(id uint, password string) error {
	if id == 0 {
		return fmt.Errorf("id is empty")
	}
	if password == "" {
		return fmt.Errorf("password is empty")
	}

	db, err := config.DB()
	if err != nil {
		return err
	}

	return db.Model(&User{}).Where("id = ?", id).Update("password", password).Error
}

func UpdateUserLastLoginDevice(id uint, deviceModel string, iosVersion string, loggedInAt time.Time) error {
	if id == 0 {
		return fmt.Errorf("id is empty")
	}
	if loggedInAt.IsZero() {
		loggedInAt = time.Now()
	}

	db, err := config.DB()
	if err != nil {
		return err
	}

	updates := map[string]any{
		"last_login_at": loggedInAt,
	}
	if deviceModel != "" {
		updates["last_login_device_model"] = trimDeviceField(deviceModel, 64)
	}
	if iosVersion != "" {
		updates["last_login_ios_version"] = trimDeviceField(iosVersion, 32)
	}

	result := db.Model(&User{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func UpdateUserLastAppEnter(id uint, enteredAt time.Time, systemLanguage string, appVersion string) error {
	if id == 0 {
		return fmt.Errorf("id is empty")
	}
	if enteredAt.IsZero() {
		enteredAt = time.Now()
	}

	db, err := config.DB()
	if err != nil {
		return err
	}

	updates := map[string]any{
		"last_app_enter_at": enteredAt,
	}
	if systemLanguage != "" {
		updates["last_system_language"] = systemLanguage
	}
	if appVersion != "" {
		updates["last_app_version"] = trimDeviceField(appVersion, 32)
	}

	result := db.Model(&User{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func trimDeviceField(value string, maxLen int) string {
	value = strings.TrimSpace(value)
	if maxLen <= 0 {
		return value
	}
	runes := []rune(value)
	if len(runes) <= maxLen {
		return value
	}
	return string(runes[:maxLen])
}

func MarkUserAccountDeletedByID(id uint) error {
	if id == 0 {
		return fmt.Errorf("id is empty")
	}

	deletedAccount := fmt.Sprintf("del:%d:%d", id, time.Now().Unix())

	return config.WithTx(func(tx *gorm.DB) error {
		result := tx.Model(&User{}).Where("id = ?", id).Update("account", deletedAccount)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}

		return tx.Delete(&User{}, id).Error
	})
}

func DeleteUserByID(id uint) error {
	if id == 0 {
		return fmt.Errorf("id is empty")
	}

	return config.Delete(&User{}, "id = ?", id)
}

func ExampleCreateUser() error {
	user, err := CreateUser("test001", "123456")
	if err != nil {
		return err
	}

	fmt.Printf("create user success, id=%d, account=%s\n", user.ID, user.Account)

	foundUser, err := GetUserByAccount("test001")
	if err != nil {
		return err
	}
	fmt.Printf("query user success, id=%d, account=%s\n", foundUser.ID, foundUser.Account)

	if err := UpdateUserPasswordByID(foundUser.ID, "new-password-123456"); err != nil {
		return err
	}
	fmt.Printf("update user password success, id=%d\n", foundUser.ID)

	if err := DeleteUserByID(foundUser.ID); err != nil {
		return err
	}
	fmt.Printf("delete user success, id=%d\n", foundUser.ID)

	return nil
}

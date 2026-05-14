package mysqlmodel

import (
	"fmt"
	"spider-server/mysql/config"
	"time"

	"gorm.io/gorm"
)

type User struct {
	ID        uint   `gorm:"primaryKey;autoIncrement"`
	Account   string `gorm:"size:64;uniqueIndex;not null"`
	Password  string `gorm:"size:255;not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
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

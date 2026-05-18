package repository

import (
	"sync/atomic"
	"time"

	"gorm.io/gorm"
)

var accountExists atomic.Bool

type User struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	Username  string    `json:"username" gorm:"not null;uniqueIndex"`
	Password  string    `json:"password" gorm:"not null"`
}

func GetUser(db *gorm.DB, username string) (*User, error) {
	var user User
	err := db.Where("username = ?", username).First(&user).Error
	return &user, err
}

func CreateUser(db *gorm.DB, username, hashedPassword string) (*User, error) {
	user := &User{Username: username, Password: hashedPassword}
	err := db.Create(user).Error
	return user, err
}

func AccountExists(db *gorm.DB) (bool, error) {
	if accountExists.Load() {
		return true, nil
	}

	var count int64
	err := db.Model(&User{}).Count(&count).Error
	if err != nil {
		return false, err
	}

	if count > 0 {
		accountExists.Store(true)
	}

	return count > 0, nil
}

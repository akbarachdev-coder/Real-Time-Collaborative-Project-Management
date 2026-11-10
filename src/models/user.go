package models

import "time"

type User struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:100;not null" json:"name"`
	Email     string    `gorm:"size:191;uniqueIndex;not null" json:"email"`
	Password  string    `gorm:"size:191;not null" json:"-"`
	Avatar    string    `gorm:"size:500" json:"avatar"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

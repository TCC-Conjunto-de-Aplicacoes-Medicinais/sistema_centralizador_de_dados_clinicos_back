package database

import (
	"time"
)

type PatientToken struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	PatientID string    `gorm:"column:patient_id;type:char(36);not null" json:"patient_id"`
	TokenCode string    `gorm:"column:token_code;type:varchar(6);not null" json:"token_code"`
	ExpiresAt time.Time `gorm:"column:expires_at;type:timestamp;not null" json:"expires_at"`
	Used      bool      `gorm:"column:used;default:false" json:"used"`
	CreatedAt time.Time `gorm:"column:created_at;type:timestamp;not null;default:CURRENT_TIMESTAMP" json:"created_at"`

	Patient Patient `gorm:"foreignKey:PatientID;references:Id" json:"-"`
}

func (PatientToken) TableName() string {
	return "patient_token"
}

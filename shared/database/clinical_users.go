package database

import (
	"time"

	"gorm.io/gorm"
)

type ClinicalUser struct {
	gorm.Model
	ClinicID     string    `gorm:"column:clinic_id;type:char(36);not null" json:"clinic_id"`
	Email        string    `gorm:"column:email;type:varchar(150);uniqueIndex;not null" json:"email"`
	PasswordHash string    `gorm:"column:password_hash;type:varchar(255);not null" json:"-"`
	FullName     string    `gorm:"column:full_name;type:varchar(100);not null" json:"full_name"`
	Role         string    `gorm:"column:role;type:varchar(50);not null" json:"role"`
	Active       bool      `gorm:"column:active;default:true" json:"active"`
	CreatedAt    time.Time `gorm:"type:timestamp;not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt    time.Time `gorm:"type:timestamp" json:"updated_at"`

	Clinic Clinic `gorm:"foreignKey:ClinicID;references:ID" json:"clinic,omitempty"`
}

func (ClinicalUser) TableName() string {
	return "clinical_user"
}

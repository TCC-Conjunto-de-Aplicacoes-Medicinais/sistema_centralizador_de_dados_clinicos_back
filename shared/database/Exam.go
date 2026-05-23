package database

import (
	"time"

	"gorm.io/gorm"
)

type Exam struct {
	gorm.Model
	Id          string    `gorm:"column:id;type:char(36);primaryKey" json:"id"`
	PatientId   string    `gorm:"column:patient_id;type:char(36);not null" json:"patient_id"`
	ClinicId    string    `gorm:"column:clinic_id;type:char(36)" json:"clinic_id"`
	LinkBucket  string    `gorm:"column:link_bucket;type:varchar(255);not null" json:"link_bucket"`
	IdCassandra string    `gorm:"column:id_cassandra;type:char(36);not null" json:"id_cassandra"`
	FlagActive  bool      `gorm:"column:flag_active;type:boolean;not null;default:true" json:"flag_active"`
	Title       string    `gorm:"column:title;type:varchar(150);not null;default:''" json:"title"`
	Provider    string    `gorm:"column:provider;type:varchar(150);default:''" json:"provider"`
	Result      string    `gorm:"column:result;type:text" json:"result"`
	CreatedAt   time.Time `gorm:"type:timestamp;not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt   time.Time `gorm:"type:timestamp;" json:"updated_at"`

	Patient     Patient            `gorm:"foreignKey:PatientId;references:Id" json:"patient"`
	Clinic      Clinic             `gorm:"foreignKey:ClinicId;references:ID" json:"clinic"`
	Permissions []DoctorPermission `gorm:"foreignKey:ExamID;references:Id" json:"permissions,omitempty"`
}

func (Exam) TableName() string {
	return "exam"
}

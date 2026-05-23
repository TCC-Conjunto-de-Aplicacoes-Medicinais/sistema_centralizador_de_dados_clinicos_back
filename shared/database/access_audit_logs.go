package database

import (
	"time"
)

type AccessAuditLog struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	ClinicID         string    `gorm:"column:clinic_id;type:char(36);not null" json:"clinic_id"`
	ClinicName       string    `gorm:"column:clinic_name;type:varchar(150);not null" json:"clinic_name"`
	RequesterEmail   string    `gorm:"column:requester_email;type:varchar(150);not null" json:"requester_email"`
	PatientID        string    `gorm:"column:patient_id;type:char(36);not null" json:"patient_id"`
	PatientName      string    `gorm:"column:patient_name;type:varchar(150);not null" json:"patient_name"`
	RequestType      string    `gorm:"column:request_type;type:varchar(20);not null" json:"request_type"`
	AuthMethod       string    `gorm:"column:auth_method;type:varchar(30);not null" json:"auth_method"`
	Justification    string    `gorm:"column:justification;type:text" json:"justification,omitempty"`
	RequesterDetails string    `gorm:"column:requester_details;type:varchar(255)" json:"requester_details,omitempty"`
	CreatedAt        time.Time `gorm:"column:created_at;type:timestamp;not null;default:CURRENT_TIMESTAMP" json:"created_at"`

	Clinic  Clinic  `gorm:"foreignKey:ClinicID;references:ID" json:"-"`
	Patient Patient `gorm:"foreignKey:PatientID;references:Id" json:"-"`
}

func (AccessAuditLog) TableName() string {
	return "access_audit_log"
}

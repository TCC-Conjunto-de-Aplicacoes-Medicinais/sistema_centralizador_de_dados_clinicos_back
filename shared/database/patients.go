package database

import (
	"time"

	"gorm.io/gorm"
)

type Patient struct {
	gorm.Model
	Id               string           `gorm:"type:char(36);primaryKey" json:"id"`
	Name             string           `gorm:"type:varchar(150);not null" json:"name"`
	CPF              string           `gorm:"type:varchar(14);uniqueIndex;not null" json:"cpf"`
	BirthDate        time.Time        `gorm:"type:date;not null" json:"birth_date"`
	Phones           []PatientPhone   `gorm:"foreignKey:PatientID;references:Id" json:"phones,omitempty"`
	Emails           []PatientEmail   `gorm:"foreignKey:PatientID;references:Id" json:"emails,omitempty"`
	Gender           string           `gorm:"type:varchar(20)" json:"gender"`
	Addresses        []PatientAddress `gorm:"foreignKey:PatientID;references:Id" json:"addresses,omitempty"`
	EmergencyContact string           `gorm:"type:varchar(20)" json:"emergency_contact"`
	Insurance        string           `gorm:"type:varchar(120)" json:"insurance"`
	BloodType        string           `gorm:"type:varchar(10)" json:"blood_type"`
	Allergies        string           `gorm:"type:text" json:"allergies"`
	CreatedAt        time.Time        `gorm:"type:timestamp;not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt        time.Time        `gorm:"type:timestamp;" json:"updated_at"`

	KeycloakID *string `gorm:"type:varchar(36);uniqueIndex" json:"keycloak_id,omitempty"`

	Verify           bool   `gorm:"type:boolean;not null;default:false" json:"verify"`
	VerificationCode string `gorm:"type:varchar(10)" json:"verification_code,omitempty"`

	Exams []Exam `gorm:"foreignKey:PatientId;references:Id" json:"exams,omitempty"`
}

func (Patient) TableName() string {
	return "patient"
}

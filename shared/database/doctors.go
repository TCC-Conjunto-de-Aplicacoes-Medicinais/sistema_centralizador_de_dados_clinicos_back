package database

import (
	"time"

	"gorm.io/gorm"
)

type Doctor struct {
	gorm.Model
	Id        string    `gorm:"type:char(36);primaryKey" json:"id"`
	Name      string    `gorm:"type:varchar(150);not null" json:"name"`
	CPF       string    `gorm:"type:varchar(14);uniqueIndex;not null" json:"cpf"`
	CRM       string    `gorm:"type:varchar(20);uniqueIndex;not null" json:"crm"`
	Phone     string    `gorm:"type:varchar(20)" json:"phone"`
	Email     string    `gorm:"type:varchar(150);uniqueIndex" json:"email"`
	Gender    string    `gorm:"type:varchar(20)" json:"gender"`
	RawData   []byte    `gorm:"type:json" json:"raw_data"`
	CreatedAt time.Time `gorm:"type:timestamp;not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt time.Time `gorm:"type:timestamp;" json:"updated_at"`
	DeletedAt time.Time `gorm:"type:timestamp;" json:"deleted_at"`

	KeycloakID *string `gorm:"type:varchar(36);uniqueIndex" json:"keycloak_id,omitempty"`

	Clinics     []Clinic           `gorm:"many2many:clinic_doctor;foreignKey:Id;joinForeignKey:DoctorID;References:ID;joinReferences:ClinicID" json:"clinics,omitempty"`
	Permissions []DoctorPermission `gorm:"foreignKey:DoctorID;references:Id" json:"permissions,omitempty"`
}

func (Doctor) TableName() string {
	return "doctor"
}

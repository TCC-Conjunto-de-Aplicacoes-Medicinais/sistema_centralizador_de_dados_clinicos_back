package database

import (
	"time"

	"gorm.io/gorm"
)

type Clinic struct {
	gorm.Model
	ID               string    `gorm:"column:id;type:char(36);primaryKey" json:"id"`
	CNPJ             string    `gorm:"column:cnpj;type:varchar(20);uniqueIndex;not null" json:"cnpj"`
	Email            string    `gorm:"column:email;type:varchar(100);uniqueIndex;not null" json:"email"`
	ResponsibleName  string    `gorm:"column:responsible_name;type:varchar(100);not null" json:"nome_responsavel"`
	ClinicName       string    `gorm:"column:clinic_name;type:varchar(100);not null" json:"nome_clinica"`
	Location         string    `gorm:"column:location;type:varchar(255)" json:"localizacao"`
	Specialty        string    `gorm:"column:specialty;type:varchar(100)" json:"especialidade"`
	Phone            string    `gorm:"column:phone;type:varchar(20)" json:"telefone"`
	BucketObj        string    `gorm:"column:bucket_obj;type:varchar(255)" json:"bucket_obj"`
	Verify           bool      `gorm:"column:verify;default:0" json:"verify"`
	VerificationCode string    `gorm:"column:verification_code;type:varchar(10)" json:"-"`
	CreatedAt        time.Time `gorm:"type:timestamp;not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt        time.Time `gorm:"type:timestamp;" json:"updated_at"`
	DeletedAt        time.Time `gorm:"type:timestamp;" json:"deleted_at"`

	KeycloakID *string `gorm:"type:varchar(36);uniqueIndex" json:"keycloak_id,omitempty"`

	Doctors []Doctor `gorm:"many2many:clinic_doctor;foreignKey:ID;joinForeignKey:ClinicID;References:Id;joinReferences:DoctorID" json:"doctors,omitempty"`
	Exams   []Exam   `gorm:"foreignKey:ClinicId;references:ID" json:"exams,omitempty"`
}

func (Clinic) TableName() string {
	return "clinic"
}

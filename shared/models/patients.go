package models

import (
	"time"

	"gorm.io/gorm"
)

type Patients struct {
	gorm.Model
	Name           string    `gorm:"type:varchar(150);not null" json:"name"`
	CPF            string    `gorm:"type:varchar(14);uniqueIndex;not null" json:"cpf"`
	BirthDate      time.Time `gorm:"type:date;not null" json:"birth_date"`
	Phone          string    `gorm:"type:varchar(20)" json:"phone"`
	Email          string    `gorm:"type:varchar(150);uniqueIndex" json:"email"`
	Gender         string    `gorm:"type:varchar(20)" json:"gender"`
	Address        string    `gorm:"type:varchar(255)" json:"address"`
	
	KeycloakID     *string   `gorm:"type:varchar(36);uniqueIndex" json:"keycloak_id,omitempty"`
}

func (Patients) TableName() string {
	return "patients"
}

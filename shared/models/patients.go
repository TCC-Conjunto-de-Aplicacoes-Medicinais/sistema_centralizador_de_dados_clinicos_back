package models

import (
	"time"

	"gorm.io/gorm"
)

// Paciente representa a entidade do banco de dados relacional (MariaDB)
type Patients struct {
	// O gorm.Model injeta automaticamente os campos ID (uint), CreatedAt, UpdatedAt e DeletedAt
	gorm.Model
	Name           string    `gorm:"type:varchar(150);not null" json:"name"`
	CPF            string    `gorm:"type:varchar(14);uniqueIndex;not null" json:"cpf"`
	BirthDate      time.Time `gorm:"type:date;not null" json:"birth_date"`
	Phone          string    `gorm:"type:varchar(20)" json:"phone"`
	Email          string    `gorm:"type:varchar(150);uniqueIndex" json:"email"`
	Gender         string    `gorm:"type:varchar(20)" json:"gender"`
	Address        string    `gorm:"type:varchar(255)" json:"address"`
	
	// Caso o paciente acesse o Keycloak no futuro para pegar resultados de exames
	KeycloakID     *string   `gorm:"type:varchar(36);uniqueIndex" json:"keycloak_id,omitempty"`
}

// TableName sobreescreve o nome estrutural do plural padrão americano do GORM
func (Patients) TableName() string {
	return "patients"
}

package database

import "time"

// PatientAccessCode é o código de 6 dígitos que o paciente gera no app para uma clínica liberar
// o histórico por um prazo curto. Só o hash é gravado; o código vale uma vez.
type PatientAccessCode struct {
	ID        string     `gorm:"column:id;type:char(36);primaryKey" json:"id"`
	PatientID string     `gorm:"column:patient_id;type:char(36);index;not null" json:"patient_id"`
	CodeHash  string     `gorm:"column:code_hash;type:char(64);not null" json:"-"`
	ExpiresAt time.Time  `gorm:"column:expires_at;type:timestamp;not null" json:"expires_at"`
	Attempts  int        `gorm:"column:attempts;not null;default:0" json:"-"`
	UsedAt    *time.Time `gorm:"column:used_at;type:timestamp" json:"-"`
	CreatedAt time.Time  `gorm:"type:timestamp;not null;default:CURRENT_TIMESTAMP"`
}

func (PatientAccessCode) TableName() string {
	return "patient_access_code"
}

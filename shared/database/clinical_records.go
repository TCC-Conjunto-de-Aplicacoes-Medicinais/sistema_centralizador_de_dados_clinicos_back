package database

import "time"

// PatientConsent é a autorização do paciente para que clínicas leiam seu histórico federado.
// Só há leitura por clínicas enquanto existir consentimento ativo (RevokedAt nulo e válido).
type PatientConsent struct {
	ID         string     `gorm:"column:id;type:char(36);primaryKey" json:"id"`
	PatientID  string     `gorm:"column:patient_id;type:char(36);index;not null" json:"patient_id"`
	Scope      string     `gorm:"column:scope;type:varchar(100);not null" json:"scope"`
	GrantedAt  time.Time  `gorm:"column:granted_at;type:timestamp;not null" json:"granted_at"`
	ValidUntil *time.Time `gorm:"column:valid_until;type:timestamp" json:"valid_until,omitempty"`
	RevokedAt  *time.Time `gorm:"column:revoked_at;type:timestamp" json:"revoked_at,omitempty"`
	CreatedAt  time.Time  `gorm:"type:timestamp;not null;default:CURRENT_TIMESTAMP"`
}

func (PatientConsent) TableName() string {
	return "patient_consent"
}

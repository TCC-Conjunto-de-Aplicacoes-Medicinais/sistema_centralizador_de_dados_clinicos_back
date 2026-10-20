package database

import "time"

// PatientPasskey é a credencial WebAuthn (passkey) do paciente. CredentialJSON guarda o
// webauthn.Credential serializado (chave pública, contador de assinaturas, transports).
type PatientPasskey struct {
	ID             string     `gorm:"column:id;type:char(36);primaryKey" json:"id"`
	PatientID      string     `gorm:"column:patient_id;type:char(36);index;not null" json:"patient_id"`
	CredentialID   string     `gorm:"column:credential_id;type:varchar(512);uniqueIndex;not null" json:"-"`
	CredentialJSON string     `gorm:"column:credential_json;type:text;not null" json:"-"`
	CreatedAt      time.Time  `gorm:"type:timestamp;not null;default:CURRENT_TIMESTAMP"`
	LastUsedAt     *time.Time `gorm:"column:last_used_at;type:timestamp"`
}

func (PatientPasskey) TableName() string {
	return "patient_passkey"
}

// PatientRecovery guarda a recuperação de passkey: o código de 6 dígitos enviado por e-mail e,
// depois de validado, o token de uso único que autoriza cadastrar uma passkey nova.
// Só os hashes são gravados. Uma linha por paciente.
type PatientRecovery struct {
	PatientID      string     `gorm:"column:patient_id;type:char(36);primaryKey" json:"patient_id"`
	CodeHash       *string    `gorm:"column:code_hash;type:char(64)" json:"-"`
	CodeExpiresAt  *time.Time `gorm:"column:code_expires_at;type:timestamp" json:"-"`
	CodeAttempts   int        `gorm:"column:code_attempts;not null;default:0" json:"-"`
	TokenHash      *string    `gorm:"column:token_hash;type:char(64)" json:"-"`
	TokenExpiresAt *time.Time `gorm:"column:token_expires_at;type:timestamp" json:"-"`
}

func (PatientRecovery) TableName() string {
	return "patient_recovery"
}

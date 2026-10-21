package services

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/database"
	"github.com/gocql/gocql"
	"gorm.io/gorm"
)

var ErrAccessCodeInvalid = errors.New("código inválido, expirado ou já utilizado")

const (
	accessCodeTTL         = 10 * time.Minute
	accessCodeMaxAttempts = 5
)

// AccessCodeService gera o código que o paciente mostra no app e troca esse código, pela clínica,
// por um consentimento de leitura que vale até o vencimento do código.
type AccessCodeService struct {
	DB   *gorm.DB
	Sync *ClinicalSyncService
}

func NewAccessCodeService(db *gorm.DB, sync *ClinicalSyncService) *AccessCodeService {
	return &AccessCodeService{DB: db, Sync: sync}
}

// Generate cria um código novo para o paciente e invalida os anteriores que ainda estavam ativos.
func (s *AccessCodeService) Generate(ctx context.Context, patientID string) (string, time.Time, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", time.Time{}, err
	}
	code := fmt.Sprintf("%06d", n.Int64())
	now := time.Now()
	expires := now.Add(accessCodeTTL)

	if err := s.DB.WithContext(ctx).Model(&database.PatientAccessCode{}).
		Where("patient_id = ? AND used_at IS NULL", patientID).
		Update("used_at", now).Error; err != nil {
		return "", time.Time{}, fmt.Errorf("falha ao invalidar códigos anteriores: %w", err)
	}
	if err := s.DB.WithContext(ctx).Create(&database.PatientAccessCode{
		ID:        gocql.TimeUUID().String(),
		PatientID: patientID,
		CodeHash:  hashSecret(code),
		ExpiresAt: expires,
	}).Error; err != nil {
		return "", time.Time{}, fmt.Errorf("falha ao gravar código: %w", err)
	}
	return code, expires, nil
}

// Redeem valida o código informado pela clínica para o CPF do paciente. Se estiver certo, grava o
// consentimento de leitura com vencimento igual ao do código e marca o código como usado.
// Devolve o id do paciente (quando encontrado) e até quando o acesso vale.
func (s *AccessCodeService) Redeem(ctx context.Context, cpf, code string) (string, time.Time, error) {
	var p database.Patient
	if err := s.DB.WithContext(ctx).Where("cpf = ?", digitsOnly(cpf)).Take(&p).Error; err != nil {
		return "", time.Time{}, ErrAccessCodeInvalid
	}

	var row database.PatientAccessCode
	now := time.Now()
	err := s.DB.WithContext(ctx).
		Where("patient_id = ? AND used_at IS NULL AND expires_at > ?", p.Id, now).
		Order("created_at DESC").
		Take(&row).Error
	if err != nil || row.Attempts >= accessCodeMaxAttempts {
		return p.Id, time.Time{}, ErrAccessCodeInvalid
	}

	// A tentativa é contada antes da comparação, para que o mesmo código não seja testado em paralelo.
	if err := s.DB.WithContext(ctx).Model(&row).Update("attempts", gorm.Expr("attempts + 1")).Error; err != nil {
		return p.Id, time.Time{}, err
	}
	if subtle.ConstantTimeCompare([]byte(hashSecret(strings.TrimSpace(code))), []byte(row.CodeHash)) != 1 {
		return p.Id, time.Time{}, ErrAccessCodeInvalid
	}

	validUntil := row.ExpiresAt
	if _, err := s.Sync.GrantConsent(ctx, p.Id, ScopeClinicalRead, &validUntil); err != nil {
		return p.Id, time.Time{}, err
	}
	if err := s.DB.WithContext(ctx).Model(&row).Update("used_at", now).Error; err != nil {
		return p.Id, time.Time{}, err
	}
	return p.Id, validUntil, nil
}

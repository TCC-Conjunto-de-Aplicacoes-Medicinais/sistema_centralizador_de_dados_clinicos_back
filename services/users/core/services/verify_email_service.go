package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/Nerzal/gocloak/v13"
	sharedConfig "github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/config"
	"gorm.io/gorm"
)

type VerifyEmailService struct {
	DB       *gorm.DB
	Keycloak *sharedConfig.KeycloakAuth
	SMTP     *SmtpEmailService
}

func NewVerifyEmailService(db *gorm.DB, kc *sharedConfig.KeycloakAuth, smtpService *SmtpEmailService) *VerifyEmailService {
	return &VerifyEmailService{
		DB:       db,
		Keycloak: kc,
		SMTP:     smtpService,
	}
}

func (s *VerifyEmailService) SendVerificationEmail(keycloakID string) error {
	sqlDB, err := s.DB.DB()
	if err != nil {
		return fmt.Errorf("erro ao obter conexão SQL: %w", err)
	}

	// 1. Busca o paciente pelo keycloak_id usando query SQL direta
	var patientID string
	var verify bool
	err = sqlDB.QueryRow(
		`SELECT id, verify FROM patient WHERE keycloak_id = ? AND (deleted_at IS NULL OR deleted_at = '0000-00-00 00:00:00') LIMIT 1`,
		keycloakID,
	).Scan(&patientID, &verify)
	if err != nil {	
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("paciente não encontrado no banco de dados")
		}
		return fmt.Errorf("erro ao buscar paciente: %w", err)
	}

	if verify {
		return errors.New("e-mail já está verificado")
	}

	// 2. Busca o e-mail principal do paciente via SQL com fallback para o primeiro e-mail
	var principalEmail string
	err = sqlDB.QueryRow(
		`SELECT email FROM patient_email
		 WHERE patient_id = ? AND (deleted_at IS NULL OR deleted_at = '0000-00-00 00:00:00')
		 ORDER BY principal DESC, id ASC
		 LIMIT 1`,
		patientID,
	).Scan(&principalEmail)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("nenhum e-mail associado a este paciente")
		}
		return fmt.Errorf("erro ao buscar e-mail do paciente: %w", err)
	}

	if principalEmail == "" {
		return errors.New("nenhum e-mail associado a este paciente")
	}

	// 3. Gera o código numérico de 6 dígitos
	code := fmt.Sprintf("%06d", rand.New(rand.NewSource(time.Now().UnixNano())).Intn(1000000))

	// 4. Atualiza o código de verificação no banco via SQL direto
	result, err := sqlDB.Exec(
		`UPDATE patient SET verification_code = ?, updated_at = NOW() WHERE id = ?`,
		code, patientID,
	)
	if err != nil {
		return fmt.Errorf("erro ao salvar código de verificação: %w", err)
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return errors.New("nenhum registro atualizado ao salvar código de verificação")
	}

	// 5. Dispara o e-mail via SMTP
	ctx := context.Background()
	go func() {
		_ = s.SMTP.SendVerificationCode(ctx, principalEmail, code)
	}()

	return nil
}

func (s *VerifyEmailService) VerifyCode(keycloakID string, code string) error {
	sqlDB, err := s.DB.DB()
	if err != nil {
		return fmt.Errorf("erro ao obter conexão SQL: %w", err)
	}

	// 1. Busca o paciente pelo keycloak_id usando query SQL direta
	var patientID string
	var verify bool
	var verificationCode string
	err = sqlDB.QueryRow(
		`SELECT id, verify, COALESCE(verification_code, '') FROM patient WHERE keycloak_id = ? AND (deleted_at IS NULL OR deleted_at = '0000-00-00 00:00:00') LIMIT 1`,
		keycloakID,
	).Scan(&patientID, &verify, &verificationCode)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("paciente não encontrado no banco de dados")
		}
		return fmt.Errorf("erro ao buscar paciente: %w", err)
	}

	if verify {
		return errors.New("paciente já está verificado")
	}

	if verificationCode != code {
		return errors.New("código inválido")
	}

	// 2. Atualiza verificação no banco local via SQL direto
	result, err := sqlDB.Exec(
		`UPDATE patient SET verify = true, verification_code = '', updated_at = NOW() WHERE id = ?`,
		patientID,
	)
	if err != nil {
		return fmt.Errorf("erro ao atualizar verificação no banco local: %w", err)
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return errors.New("nenhum registro atualizado na verificação")
	}

	// 3. Atualiza no Keycloak
	ctx := context.Background()
	token, err := s.Keycloak.Client.LoginClient(ctx, s.Keycloak.ClientID, s.Keycloak.ClientSecret, s.Keycloak.Realm)
	if err != nil {
		return fmt.Errorf("erro ao autenticar no keycloak: %w", err)
	}

	kcUser, err := s.Keycloak.Client.GetUserByID(ctx, token.AccessToken, s.Keycloak.Realm, keycloakID)
	if err != nil {
		return fmt.Errorf("erro ao buscar usuário no keycloak: %w", err)
	}

	kcUser.EmailVerified = gocloak.BoolP(true)

	if err := s.Keycloak.Client.UpdateUser(ctx, token.AccessToken, s.Keycloak.Realm, *kcUser); err != nil {
		return fmt.Errorf("erro ao atualizar usuário no keycloak: %w", err)
	}

	return nil
}

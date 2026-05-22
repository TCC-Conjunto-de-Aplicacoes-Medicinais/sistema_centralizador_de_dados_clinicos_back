package services

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/Nerzal/gocloak/v13"
	sharedConfig "github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/config"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/database"
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

func (s *VerifyEmailService) SendVerificationEmail(id string) error {
	var patient database.Patient
	// Pré-carrega Emails para pegar o e-mail principal
	if err := s.DB.Preload("Emails").Where("patient_id = ?", id).First(&patient).Error; err != nil {
		return errors.New("paciente não encontrado no banco de dados")
	}

	if patient.Verify {
		return errors.New("e-mail já está verificado")
	}

	var principalEmail string
	for _, e := range patient.Emails {
		if e.Principal {
			principalEmail = e.Email
			break
		}
	}
	// Fallback se não tiver e-mail principal marcado, pega o primeiro
	if principalEmail == "" && len(patient.Emails) > 0 {
		principalEmail = patient.Emails[0].Email
	}

	if principalEmail == "" {
		return errors.New("nenhum e-mail associado a este paciente")
	}

	// Gera o código numérico de 6 dígitos
	code := fmt.Sprintf("%06d", rand.New(rand.NewSource(time.Now().UnixNano())).Intn(1000000))

	// Atualiza o código no banco
	if err := s.DB.Model(&patient).Update("verification_code", code).Error; err != nil {
		return fmt.Errorf("erro ao salvar código de verificação: %w", err)
	}

	// Dispara o e-mail via SMTP
	ctx := context.Background()
	go func() {
		_ = s.SMTP.SendVerificationCode(ctx, principalEmail, code)
	}()

	return nil
}

func (s *VerifyEmailService) VerifyCode(keycloakID string, code string) error {
	var patient database.Patient
	if err := s.DB.Where("keycloak_id = ?", keycloakID).First(&patient).Error; err != nil {
		return errors.New("paciente não encontrado no banco de dados")
	}

	if patient.Verify {
		return errors.New("paciente já está verificado")
	}

	if patient.VerificationCode != code {
		return errors.New("código inválido")
	}

	// Atualiza DB local
	if err := s.DB.Model(&patient).Updates(map[string]interface{}{
		"verify":            true,
		"verification_code": "",
	}).Error; err != nil {
		return fmt.Errorf("erro ao atualizar verificação no banco local: %w", err)
	}

	// Atualiza no Keycloak
	ctx := context.Background()
	token, err := s.Keycloak.Client.LoginClient(ctx, s.Keycloak.ClientID, s.Keycloak.ClientSecret, s.Keycloak.Realm)
	if err != nil {
		return fmt.Errorf("erro ao autenticar no keycloak: %w", err)
	}

	kcUser, err := s.Keycloak.Client.GetUserByID(ctx, token.AccessToken, s.Keycloak.Realm, *patient.KeycloakID)
	if err != nil {
		return fmt.Errorf("erro ao buscar usuário no keycloak: %w", err)
	}

	kcUser.EmailVerified = gocloak.BoolP(true)
	
	if err := s.Keycloak.Client.UpdateUser(ctx, token.AccessToken, s.Keycloak.Realm, *kcUser); err != nil {
		return fmt.Errorf("erro ao atualizar usuário no keycloak: %w", err)
	}

	return nil
}

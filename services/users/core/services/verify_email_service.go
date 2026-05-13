package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/Nerzal/gocloak/v13"
	sharedConfig "github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/config"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/database"
	"gorm.io/gorm"
)

type VerifyEmailService struct {
	DB       *gorm.DB
	Keycloak *sharedConfig.KeycloakAuth
}

func NewVerifyEmailService(db *gorm.DB, kc *sharedConfig.KeycloakAuth) *VerifyEmailService {
	return &VerifyEmailService{
		DB:       db,
		Keycloak: kc,
	}
}

func (s *VerifyEmailService) SendVerificationEmail(id string) error {
	var patient database.Patient
	if err := s.DB.Where("id = ?", id).First(&patient).Error; err != nil {
		return errors.New("paciente não encontrado no banco de dados")
	}

	ctx := context.Background()
	token, err := s.Keycloak.Client.LoginClient(ctx, s.Keycloak.ClientID, s.Keycloak.ClientSecret, s.Keycloak.Realm)
	if err != nil {
		return fmt.Errorf("erro ao autenticar no keycloak: %w", err)
	}

	actions := []string{"VERIFY_EMAIL"}
	params := gocloak.ExecuteActionsEmail{
		UserID:   &patient.Id,
		ClientID: gocloak.StringP(s.Keycloak.ClientID),
		Actions:  &actions,
	}

	err = s.Keycloak.Client.ExecuteActionsEmail(ctx, token.AccessToken, s.Keycloak.Realm, params)
	if err != nil {
		return fmt.Errorf("erro ao disparar e-mail de verificação no keycloak: %w", err)
	}

	return nil
}

package services

import (
	"context"
	"fmt"

	"github.com/Nerzal/gocloak/v13"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/services/users/internal/usecase"
	sharedConfig "github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/config"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/models"
	"github.com/gocql/gocql"
	"gorm.io/gorm"
)

type SignupService struct {
	DB           *gorm.DB
	Cassandra    *sharedConfig.DbClient
	Keycloak     *sharedConfig.KeycloakAuth
	ValidationUC *usecase.ValidateSignupUseCase
}

func NewSignupService(db *gorm.DB, cassandra *sharedConfig.DbClient, kc *sharedConfig.KeycloakAuth) *SignupService {
	return &SignupService{
		DB:           db,
		Cassandra:    cassandra,
		Keycloak:     kc,
		ValidationUC: usecase.NewValidateSignupUseCase(db, kc),
	}
}

func (s *SignupService) Signup(req models.SignupRequest) error {
	ctx := context.Background()

	if err := s.ValidationUC.Execute(ctx, req); err != nil {
		return err
	}

	token, err := s.Keycloak.Client.LoginClient(ctx, s.Keycloak.ClientID, s.Keycloak.ClientSecret, s.Keycloak.Realm)
	if err != nil {
		return fmt.Errorf("erro ao autenticar no keycloak: %w", err)
	}

	enabled := true
	keycloakUser := gocloak.User{
		Username:  gocloak.StringP(req.Email),
		Email:     gocloak.StringP(req.Email),
		FirstName: gocloak.StringP(req.Name),
		Enabled:   &enabled,
	}

	keycloakID, err := s.Keycloak.Client.CreateUser(ctx, token.AccessToken, s.Keycloak.Realm, keycloakUser)
	if err != nil {
		return fmt.Errorf("erro ao criar usuário no keycloak: %w", err)
	}

	err = s.Keycloak.Client.SetPassword(ctx, token.AccessToken, keycloakID, s.Keycloak.Realm, req.Password, false)
	if err != nil {
		return fmt.Errorf("erro ao definir senha no keycloak: %w", err)
	}

	patient := models.Patients{
		Name:       req.Name,
		Email:      req.Email,
		CPF:        req.CPF,
		KeycloakID: &keycloakID,
	}

	if result := s.DB.Create(&patient); result.Error != nil {
		return fmt.Errorf("erro ao salvar paciente no banco relacional: %w", result.Error)
	}

	uuid, err := gocql.ParseUUID(keycloakID)
	if err != nil {
		return fmt.Errorf("erro ao converter id do keycloak para uuid: %w", err)
	}

	query := `INSERT INTO user_devices (user_id, device_name, public_key) VALUES (?, ?, ?)`
	if err := s.Cassandra.Core.Query(query, uuid, req.Device.DeviceName, req.Device.PublicKey).Exec(); err != nil {
		return fmt.Errorf("erro ao salvar dispositivo no cassandra: %w", err)
	}

	return nil
}

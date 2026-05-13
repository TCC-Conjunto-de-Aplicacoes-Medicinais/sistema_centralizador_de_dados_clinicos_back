package services

import (
	"context"
	"fmt"
	"time"

	"github.com/Nerzal/gocloak/v13"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/services/users/core/usecase"
	sharedConfig "github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/config"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/database"
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
		Username:      gocloak.StringP(req.CPF),
		Email:         gocloak.StringP(req.Email),
		FirstName:     gocloak.StringP(req.Name),
		LastName:      gocloak.StringP("-"),
		EmailVerified: gocloak.BoolP(false),
		Enabled:       &enabled,
	}

	keycloakID, err := s.Keycloak.Client.CreateUser(ctx, token.AccessToken, s.Keycloak.Realm, keycloakUser)
	if err != nil {
		return fmt.Errorf("erro ao criar usuário no keycloak: %w", err)
	}

	err = s.Keycloak.Client.SetPassword(ctx, token.AccessToken, keycloakID, s.Keycloak.Realm, req.Password, false)
	if err != nil {
		return fmt.Errorf("erro ao definir senha no keycloak: %w", err)
	}

	uuid, err := gocql.ParseUUID(keycloakID)
	if err != nil {
		return fmt.Errorf("erro ao converter id do keycloak para uuid: %w", err)
	}

	patient := database.Patient{
		Id:         uuid.String(),
		Name:       req.Name,
		CPF:        req.CPF,
		KeycloakID: &keycloakID,
		Emails: []database.PatientEmail{
			{
				Id:        gocql.TimeUUID().String(),
				Email:     req.Email,
				Principal: true,
			},
		},
	}

	if result := s.DB.Create(&patient); result.Error != nil {
		return fmt.Errorf("erro ao salvar paciente no banco relacional: %w", result.Error)
	}

	query := `INSERT INTO user_devices (user_id, created_at, device_name, public_key) VALUES (?, ?, ?, ?)`
	if err := s.Cassandra.Core.Query(query, uuid, time.Now().Unix(), req.Device.DeviceName, req.Device.PublicKey).Exec(); err != nil {
		return fmt.Errorf("erro ao salvar dispositivo no cassandra: %w", err)
	}

	return nil
}

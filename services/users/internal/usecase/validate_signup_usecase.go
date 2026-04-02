package usecase

import (
	"context"
	"errors"

	sharedConfig "github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/config"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/models"

	"github.com/Nerzal/gocloak/v13"
	"gorm.io/gorm"
)

type ValidateSignupUseCase struct {
	DB       *gorm.DB
	Keycloak *sharedConfig.KeycloakAuth
}

func NewValidateSignupUseCase(db *gorm.DB, kc *sharedConfig.KeycloakAuth) *ValidateSignupUseCase {
	return &ValidateSignupUseCase{DB: db, Keycloak: kc}
}

func (uc *ValidateSignupUseCase) Execute(ctx context.Context, req models.SignupRequest) error {
	var count int64

	if err := uc.DB.Model(&models.Patients{}).Where("cpf = ?", req.CPF).Count(&count).Error; err != nil {
		return errors.New("erro ao verificar integridade do CPF no banco de dados")
	}
	if count > 0 {
		return errors.New("validação falhou: o CPF informado já está em uso")
	}

	if err := uc.DB.Model(&models.Patients{}).Where("email = ?", req.Email).Count(&count).Error; err != nil {
		return errors.New("erro ao verificar integridade do E-mail no banco de dados")
	}
	if count > 0 {
		return errors.New("validação falhou: o E-mail informado já está em uso")
	}

	token, err := uc.Keycloak.Client.LoginClient(ctx, uc.Keycloak.ClientID, uc.Keycloak.ClientSecret, uc.Keycloak.Realm)
	if err == nil {
		users, _ := uc.Keycloak.Client.GetUsers(ctx, token.AccessToken, uc.Keycloak.Realm, gocloak.GetUsersParams{
			Email: gocloak.StringP(req.Email),
			Exact: gocloak.BoolP(true),
		})

		if len(users) > 0 {
			return errors.New("validação falhou: uma conta de acesso vinculada a este e-mail já existe no Keycloak")
		}
	}

	return nil
}

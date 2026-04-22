package services

import (
	"context"
	"fmt"

	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/services/users/core/usecase"
	sharedConfig "github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/config"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/models"
)

type LoginService struct {
	Keycloak    *sharedConfig.KeycloakAuth
	DPoPUseCase *usecase.ValidateDPoPUseCase
}

func NewLoginService(kc *sharedConfig.KeycloakAuth, dpopUC *usecase.ValidateDPoPUseCase) *LoginService {
	return &LoginService{
		Keycloak:    kc,
		DPoPUseCase: dpopUC,
	}
}

func (s *LoginService) Login(proofJWT string, req models.LoginRequest) (*models.LoginResponse, error) {
	if err := s.DPoPUseCase.Execute(proofJWT); err != nil {
		return nil, err
	}

	ctx := context.Background()
	jwt, err := s.Keycloak.Client.Login(
		ctx,
		s.Keycloak.ClientID,
		s.Keycloak.ClientSecret,
		s.Keycloak.Realm,
		req.Email,
		req.Password,
	)
	if err != nil {
		return nil, fmt.Errorf("credenciais inválidas: %w", err)
	}

	return &models.LoginResponse{
		AccessToken:  jwt.AccessToken,
		TokenType:    "DPoP",
		ExpiresIn:    jwt.ExpiresIn,
		RefreshToken: jwt.RefreshToken,
	}, nil
}

func (s *LoginService) Refresh(proofJWT string, req models.RefreshRequest) (*models.RefreshResponse, error) {
	if err := s.DPoPUseCase.Execute(proofJWT); err != nil {
		return nil, err
	}

	ctx := context.Background()
	jwt, err := s.Keycloak.Client.RefreshToken(
		ctx,
		req.RefreshToken,
		s.Keycloak.ClientID,
		s.Keycloak.ClientSecret,
		s.Keycloak.Realm,
	)
	if err != nil {
		return nil, fmt.Errorf("erro ao revalidar sessão: %w", err)
	}

	return &models.RefreshResponse{
		AccessToken:  jwt.AccessToken,
		TokenType:    "DPoP",
		ExpiresIn:    jwt.ExpiresIn,
		RefreshToken: jwt.RefreshToken,
	}, nil
}

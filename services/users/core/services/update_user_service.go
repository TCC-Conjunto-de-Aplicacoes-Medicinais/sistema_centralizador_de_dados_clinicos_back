package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/Nerzal/gocloak/v13"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/services/users/core/usecase"
	sharedConfig "github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/config"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/database"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/models"
	"gorm.io/gorm"
)

type UpdateUserService struct {
	DB          *gorm.DB
	Keycloak    *sharedConfig.KeycloakAuth
	DPoPUseCase *usecase.ValidateDPoPUseCase
}

func NewUpdateUserService(db *gorm.DB, kc *sharedConfig.KeycloakAuth, dpopUC *usecase.ValidateDPoPUseCase) *UpdateUserService {
	return &UpdateUserService{
		DB:          db,
		Keycloak:    kc,
		DPoPUseCase: dpopUC,
	}
}

func (s *UpdateUserService) UpdateUser(proofJWT, id string, req models.UpdateUserRequest) error {
	if err := s.DPoPUseCase.Execute(proofJWT); err != nil {
		return err
	}

	var patient database.Patients
	if err := s.DB.Where("id = ?", id).First(&patient).Error; err != nil {
		return errors.New("paciente não encontrado no banco de dados")
	}

	updates := map[string]interface{}{}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Phone != "" {
		updates["phone"] = req.Phone
	}
	if req.Address != "" {
		updates["address"] = req.Address
	}
	if req.EmergencyContact != "" {
		updates["emergency_contact"] = req.EmergencyContact
	}

	if len(updates) == 0 {
		return errors.New("nenhum dado para atualizar")
	}

	if err := s.DB.Model(&patient).Updates(updates).Error; err != nil {
		return fmt.Errorf("erro ao atualizar dados no mariadb: %w", err)
	}

	// Update Keycloak if name changed
	if req.Name != "" && patient.KeycloakID != nil {
		ctx := context.Background()
		token, err := s.Keycloak.Client.LoginClient(ctx, s.Keycloak.ClientID, s.Keycloak.ClientSecret, s.Keycloak.Realm)
		if err == nil {
			kcUser, err := s.Keycloak.Client.GetUserByID(ctx, token.AccessToken, s.Keycloak.Realm, *patient.KeycloakID)
			if err == nil && kcUser != nil {
				kcUser.FirstName = gocloak.StringP(req.Name)
				_ = s.Keycloak.Client.UpdateUser(ctx, token.AccessToken, s.Keycloak.Realm, *kcUser)
			}
		}
	}

	return nil
}

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
	"github.com/gocql/gocql"
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

func (s *UpdateUserService) UpdateUser(id string, req models.UpdateUserRequest) error {
	var patient database.Patients
	if err := s.DB.Where("keycloak_id = ?", id).First(&patient).Error; err != nil {
		return errors.New("paciente não encontrado no banco de dados")
	}

	// Inicia transação para garantir atomicidade entre as tabelas
	tx := s.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 1. Atualiza dados básicos na tabela principal patients
	updates := map[string]interface{}{}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.EmergencyContact != "" {
		updates["emergency_contact"] = req.EmergencyContact
	}

	if len(updates) > 0 {
		if err := tx.Model(&patient).Updates(updates).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("erro ao atualizar dados básicos: %w", err)
		}
	}

	var principalEmail string

	// 2. Atualiza Coleção de E-mails (Lógica de substituição total para a coleção)
	if len(req.Emails) > 0 {
		if err := tx.Where("patient_id = ?", patient.Id).Delete(&database.PatientEmail{}).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("erro ao limpar e-mails antigos: %w", err)
		}
		for _, e := range req.Emails {
			newEmail := database.PatientEmail{
				Id:        gocql.TimeUUID().String(),
				PatientID: patient.Id,
				Email:     e.Email,
				Principal: e.Principal,
			}
			if err := tx.Create(&newEmail).Error; err != nil {
				tx.Rollback()
				return fmt.Errorf("erro ao salvar novo e-mail: %w", err)
			}
			if e.Principal {
				principalEmail = e.Email
			}
		}
	}

	// 3. Atualiza Coleção de Telefones
	if len(req.Phones) > 0 {
		if err := tx.Where("patient_id = ?", patient.Id).Delete(&database.PatientPhone{}).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("erro ao limpar telefones antigos: %w", err)
		}
		for _, p := range req.Phones {
			newPhone := database.PatientPhone{
				Id:        gocql.TimeUUID().String(),
				PatientID: patient.Id,
				Phone:     p.Phone,
				Principal: p.Principal,
			}
			if err := tx.Create(&newPhone).Error; err != nil {
				tx.Rollback()
				return fmt.Errorf("erro ao salvar novo telefone: %w", err)
			}
		}
	}

	// 4. Atualiza Coleção de Endereços
	if len(req.Addresses) > 0 {
		if err := tx.Where("patient_id = ?", patient.Id).Delete(&database.PatientAddress{}).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("erro ao limpar endereços antigos: %w", err)
		}
		for _, a := range req.Addresses {
			newAddr := database.PatientAddress{
				Id:        gocql.TimeUUID().String(),
				PatientID: patient.Id,
				Address:   a.Address,
				Principal: a.Principal,
			}
			if err := tx.Create(&newAddr).Error; err != nil {
				tx.Rollback()
				return fmt.Errorf("erro ao salvar novo endereço: %w", err)
			}
		}
	}

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("erro ao confirmar alterações no banco: %w", err)
	}

	// 5. Sincroniza com Keycloak se necessário (Nome ou E-mail Principal)
	if (req.Name != "" || principalEmail != "") && patient.KeycloakID != nil {
		ctx := context.Background()
		token, err := s.Keycloak.Client.LoginClient(ctx, s.Keycloak.ClientID, s.Keycloak.ClientSecret, s.Keycloak.Realm)
		if err == nil {
			kcUser, err := s.Keycloak.Client.GetUserByID(ctx, token.AccessToken, s.Keycloak.Realm, *patient.KeycloakID)
			if err == nil && kcUser != nil {
				if req.Name != "" {
					kcUser.FirstName = gocloak.StringP(req.Name)
				}
				if principalEmail != "" {
					kcUser.Email = gocloak.StringP(principalEmail)
				}
				_ = s.Keycloak.Client.UpdateUser(ctx, token.AccessToken, s.Keycloak.Realm, *kcUser)
			}
		}
	}

	return nil
}

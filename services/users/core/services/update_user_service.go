package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/Nerzal/gocloak/v13"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/services/users/core/usecase"
	sharedConfig "github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/config"
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
	var patientID string
	var keycloakIDStr string
	var currentName string

	// 1. Busca o paciente pelo keycloak_id usando query SQL direta
	err := s.DB.Raw(
		`SELECT id, COALESCE(keycloak_id, ''), name FROM patient WHERE keycloak_id = ? AND (deleted_at IS NULL OR deleted_at = '0000-00-00 00:00:00') LIMIT 1`,
		id,
	).Row().Scan(&patientID, &keycloakIDStr, &currentName)
	if err != nil {
		return errors.New("paciente não encontrado no banco de dados")
	}

	// Inicia transação para garantir atomicidade entre as tabelas
	tx := s.DB.Begin()
	if tx.Error != nil {
		return fmt.Errorf("erro ao iniciar transação: %w", tx.Error)
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 2. Atualiza dados básicos na tabela principal patient
	if req.Name != "" || req.EmergencyContact != "" {
		var setClause string
		var args []interface{}
		if req.Name != "" && req.EmergencyContact != "" {
			setClause = "name = ?, emergency_contact = ?, "
			args = append(args, req.Name, req.EmergencyContact)
		} else if req.Name != "" {
			setClause = "name = ?, "
			args = append(args, req.Name)
		} else if req.EmergencyContact != "" {
			setClause = "emergency_contact = ?, "
			args = append(args, req.EmergencyContact)
		}

		query := fmt.Sprintf("UPDATE patient SET %supdated_at = NOW() WHERE id = ?", setClause)
		args = append(args, patientID)

		if err := tx.Exec(query, args...).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("erro ao atualizar dados básicos: %w", err)
		}
	}

	var principalEmail string

	// 3. Atualiza Coleção de E-mails (Lógica de substituição total para a coleção)
	if len(req.Emails) > 0 {
		if err := tx.Exec(`UPDATE patient_email SET deleted_at = NOW() WHERE patient_id = ? AND (deleted_at IS NULL OR deleted_at = '0000-00-00 00:00:00')`, patientID).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("erro ao limpar e-mails antigos: %w", err)
		}
		for _, e := range req.Emails {
			newEmailID := gocql.TimeUUID().String()
			err := tx.Exec(
				`INSERT INTO patient_email (id, patient_id, email, principal, created_at, updated_at) VALUES (?, ?, ?, ?, NOW(), NOW())`,
				newEmailID, patientID, e.Email, e.Principal,
			).Error
			if err != nil {
				tx.Rollback()
				return fmt.Errorf("erro ao salvar novo e-mail: %w", err)
			}
			if e.Principal {
				principalEmail = e.Email
			}
		}
	}

	// 4. Atualiza Coleção de Telefones
	if len(req.Phones) > 0 {
		if err := tx.Exec(`UPDATE patient_phone SET deleted_at = NOW() WHERE patient_id = ? AND (deleted_at IS NULL OR deleted_at = '0000-00-00 00:00:00')`, patientID).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("erro ao limpar telefones antigos: %w", err)
		}
		for _, p := range req.Phones {
			newPhoneID := gocql.TimeUUID().String()
			err := tx.Exec(
				`INSERT INTO patient_phone (id, patient_id, phone, principal, created_at, updated_at) VALUES (?, ?, ?, ?, NOW(), NOW())`,
				newPhoneID, patientID, p.Phone, p.Principal,
			).Error
			if err != nil {
				tx.Rollback()
				return fmt.Errorf("erro ao salvar novo telefone: %w", err)
			}
		}
	}

	// 5. Atualiza Coleção de Endereços
	if len(req.Addresses) > 0 {
		if err := tx.Exec(`UPDATE patient_address SET deleted_at = NOW() WHERE patient_id = ? AND (deleted_at IS NULL OR deleted_at = '0000-00-00 00:00:00')`, patientID).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("erro ao limpar endereços antigos: %w", err)
		}
		for _, a := range req.Addresses {
			newAddrID := gocql.TimeUUID().String()
			err := tx.Exec(
				`INSERT INTO patient_address (id, patient_id, address, principal, created_at, updated_at) VALUES (?, ?, ?, ?, NOW(), NOW())`,
				newAddrID, patientID, a.Address, a.Principal,
			).Error
			if err != nil {
				tx.Rollback()
				return fmt.Errorf("erro ao salvar novo endereço: %w", err)
			}
		}
	}

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("erro ao confirmar alterações no banco: %w", err)
	}

	// 6. Sincroniza com Keycloak se necessário (Nome ou E-mail Principal)
	if (req.Name != "" || principalEmail != "") && keycloakIDStr != "" {
		ctx := context.Background()
		token, err := s.Keycloak.Client.LoginClient(ctx, s.Keycloak.ClientID, s.Keycloak.ClientSecret, s.Keycloak.Realm)
		if err == nil {
			kcUser, err := s.Keycloak.Client.GetUserByID(ctx, token.AccessToken, s.Keycloak.Realm, keycloakIDStr)
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

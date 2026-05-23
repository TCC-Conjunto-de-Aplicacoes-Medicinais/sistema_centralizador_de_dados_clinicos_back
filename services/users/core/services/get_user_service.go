package services

import (
	"errors"

	"gorm.io/gorm"
)

// UserProfileResponse é o DTO retornado pelo endpoint GET /api/users/profile.
type UserProfileResponse struct {
	Name    string `json:"name"`
	Phone   string `json:"phone"`
	Address string `json:"address"`
}

type GetUserService struct {
	DB *gorm.DB
}

func NewGetUserService(db *gorm.DB) *GetUserService {
	return &GetUserService{DB: db}
}

// GetProfile busca o perfil editável do paciente pelo Keycloak ID (sub do JWT).
// Retorna nome, telefone principal e endereço principal.
func (s *GetUserService) GetProfile(keycloakID string) (*UserProfileResponse, error) {
	var patientID, name string

	// 1. Busca o paciente pelo keycloak_id
	err := s.DB.Raw(
		`SELECT id, name FROM patient WHERE keycloak_id = ? AND (deleted_at IS NULL OR deleted_at = '0000-00-00 00:00:00') LIMIT 1`,
		keycloakID,
	).Row().Scan(&patientID, &name)
	if err != nil {
		return nil, errors.New("paciente não encontrado")
	}

	resp := &UserProfileResponse{
		Name: name,
	}

	// 2. Busca telefone principal ativo
	var phone string
	err = s.DB.Raw(
		`SELECT phone FROM patient_phone WHERE patient_id = ? AND principal = true AND (deleted_at IS NULL OR deleted_at = '0000-00-00 00:00:00') LIMIT 1`,
		patientID,
	).Row().Scan(&phone)
	if err == nil {
		resp.Phone = phone
	}

	// 3. Busca endereço principal ativo
	var address string
	err = s.DB.Raw(
		`SELECT address FROM patient_address WHERE patient_id = ? AND principal = true AND (deleted_at IS NULL OR deleted_at = '0000-00-00 00:00:00') LIMIT 1`,
		patientID,
	).Row().Scan(&address)
	if err == nil {
		resp.Address = address
	}

	return resp, nil
}

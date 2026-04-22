package usecase

import (
	"errors"

	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/dpop"
)

type ValidateDPoPUseCase struct {
	ReplayStore *dpop.ReplayStore
	BaseURL     string
}

func NewValidateDPoPUseCase(store *dpop.ReplayStore, baseURL string) *ValidateDPoPUseCase {
	return &ValidateDPoPUseCase{ReplayStore: store, BaseURL: baseURL}
}

// Execute valida o DPoP proof para o endpoint de login.
func (uc *ValidateDPoPUseCase) Execute(proofJWT string) error {
	if proofJWT == "" {
		return errors.New("header DPoP ausente")
	}

	expectedHTU := uc.BaseURL + "/api/login"

	jti, err := dpop.ParseAndValidate(proofJWT, "POST", expectedHTU)
	if err != nil {
		return err
	}

	return uc.ReplayStore.CheckAndStore(jti)
}

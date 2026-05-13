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

// Execute valida o DPoP proof para um endpoint específico.
func (uc *ValidateDPoPUseCase) Execute(proofJWT, htm, htu string) error {
	if proofJWT == "" {
		return errors.New("header DPoP ausente")
	}

	jti, err := dpop.ParseAndValidate(proofJWT, htm, htu)
	if err != nil {
		return err
	}

	return uc.ReplayStore.CheckAndStore(jti)
}

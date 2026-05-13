package unitTests

import (
	"context"
	"testing"

	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/services/users/core/usecase"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/models"
	"github.com/stretchr/testify/assert"
)

func TestValidateSignupUseCase_InvalidCPF(t *testing.T) {
	uc := usecase.NewValidateSignupUseCase(nil, nil)
	ctx := context.Background()

	req := models.SignupRequest{
		CPF:   "12345678900", // CPF Inválido (formato/dígito)
		Email: "test@example.com",
	}

	err := uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "formato de CPF inválido")
}

func TestValidateSignupUseCase_InvalidEmail(t *testing.T) {
	uc := usecase.NewValidateSignupUseCase(nil, nil)
	ctx := context.Background()

	req := models.SignupRequest{
		CPF:   "11144477735", // CPF Válido (matematicamente)
		Email: "email-invalido",
	}

	err := uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "formato de e-mail inválido")
}

func TestValidateSignupUseCase_AllSameDigitsCPF(t *testing.T) {
	uc := usecase.NewValidateSignupUseCase(nil, nil)
	ctx := context.Background()

	req := models.SignupRequest{
		CPF:   "11111111111", // CPF Inválido (todos iguais)
		Email: "test@example.com",
	}

	err := uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "formato de CPF inválido")
}

func TestValidateSignupUseCase_ShortCPF(t *testing.T) {
	uc := usecase.NewValidateSignupUseCase(nil, nil)
	ctx := context.Background()

	req := models.SignupRequest{
		CPF:   "123", // Muito curto
		Email: "test@example.com",
	}

	err := uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "formato de CPF inválido")
}


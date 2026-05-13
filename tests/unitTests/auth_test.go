package unitTests

import (
	"testing"

	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/auth"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
)

func TestExtractSubFromToken_Success(t *testing.T) {
	// Cria um token JWT dummy sem assinatura
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "user-123",
	})
	tokenString, _ := token.SignedString([]byte("secret"))

	authHeader := "Bearer " + tokenString
	claims, err := auth.ExtractUserClaims(authHeader)

	assert.NoError(t, err)
	assert.Equal(t, "user-123", claims.PatientID)
}

func TestExtractSubFromToken_DPoP_Success(t *testing.T) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "user-456",
		"name": "Test User",
	})
	tokenString, _ := token.SignedString([]byte("secret"))

	authHeader := "DPoP " + tokenString
	claims, err := auth.ExtractUserClaims(authHeader)

	assert.NoError(t, err)
	assert.Equal(t, "user-456", claims.PatientID)
	assert.Equal(t, "Test User", claims.Name)
}

func TestExtractSubFromToken_Failures(t *testing.T) {
	tests := []struct {
		name   string
		header string
	}{
		{"Empty Header", ""},
		{"Wrong Prefix", "Basic abc"},
		{"Invalid Format", "Bearer"},
		{"Invalid Token", "Bearer abc.def"},
		{"Missing Sub", "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.e30.s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := auth.ExtractUserClaims(tt.header)
			assert.Error(t, err)
		})
	}
}

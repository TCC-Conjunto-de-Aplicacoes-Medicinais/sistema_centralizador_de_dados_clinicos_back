package unitTests

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Nerzal/gocloak/v13"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/services/users/core/services"
	sharedConfig "github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/config"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/models"
	"github.com/stretchr/testify/assert"
)

func setupLoginMockKeycloakServer(t *testing.T) (*httptest.Server, *sharedConfig.KeycloakAuth) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "POST" && strings.Contains(r.URL.Path, "/protocol/openid-connect/token") {
			// Check for bad credentials if simulating error
			if r.FormValue("username") == "invalid-cpf" {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error": "unauthorized"}`))
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"access_token": "mock-access-token", "refresh_token": "mock-refresh-token", "expires_in": 3600}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))

	client := gocloak.NewClient(server.URL)
	auth := &sharedConfig.KeycloakAuth{
		Client:       client,
		ClientID:     "client-id",
		ClientSecret: "secret",
		Realm:        "realm",
	}
	return server, auth
}

func TestLoginService_Login_Success(t *testing.T) {
	server, kcAuth := setupLoginMockKeycloakServer(t)
	defer server.Close()

	service := services.NewLoginService(kcAuth, nil)

	req := models.LoginRequest{
		CPF:      "11144477735",
		Password: "password123",
	}

	res, err := service.Login(req)
	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, "mock-access-token", res.AccessToken)
	assert.Equal(t, "mock-refresh-token", res.RefreshToken)
	assert.Equal(t, 3600, res.ExpiresIn)
}

func TestLoginService_Login_Error(t *testing.T) {
	server, kcAuth := setupLoginMockKeycloakServer(t)
	// We'll shut down the server to force a network error
	server.Close()

	service := services.NewLoginService(kcAuth, nil)

	req := models.LoginRequest{
		CPF:      "11144477735",
		Password: "password123",
	}

	res, err := service.Login(req)
	assert.Error(t, err)
	assert.Nil(t, res)
}

func TestLoginService_Refresh_Success(t *testing.T) {
	server, kcAuth := setupLoginMockKeycloakServer(t)
	defer server.Close()

	service := services.NewLoginService(kcAuth, nil)

	req := models.RefreshRequest{
		RefreshToken: "mock-refresh-token",
	}

	res, err := service.Refresh(req)
	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, "mock-access-token", res.AccessToken)
	assert.Equal(t, "mock-refresh-token", res.RefreshToken)
	assert.Equal(t, 3600, res.ExpiresIn)
}

func TestLoginService_Refresh_Error(t *testing.T) {
	server, kcAuth := setupLoginMockKeycloakServer(t)
	server.Close()

	service := services.NewLoginService(kcAuth, nil)

	req := models.RefreshRequest{
		RefreshToken: "mock-refresh-token",
	}

	res, err := service.Refresh(req)
	assert.Error(t, err)
	assert.Nil(t, res)
}

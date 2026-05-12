package unitTests

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	userHttp "github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/services/users/core/http"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/services/users/core/services"
	
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func setupUnitRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.Default()

	userHandler := userHttp.NewUserHandler(
		&services.SignupService{},
		&services.LoginService{},
		&services.UpdateUserService{},
		&services.VerifyEmailService{},
		nil,
	)

	router.POST("/api/signup", userHandler.Signup)
	router.POST("/api/login", userHandler.Login)
	router.POST("/api/refresh", userHandler.Refresh)
	router.PUT("/api/users/:id", userHandler.UpdateUser)
	router.POST("/api/users/:id/send-verify-email", userHandler.SendVerifyEmail)
	
	return router
}

func TestSignupHandler_BindJSON_Error(t *testing.T) {
	router := setupUnitRouter()

	// Payload de teste faltando os campos obrigatórios (name, password, cpf, etc)
	payloadInvalido := []byte(`{"email": "abc@abc.com"}`)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/signup", bytes.NewBuffer(payloadInvalido))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	// Espera-se 400 Bad Request porque não conseguiu parsear o JSON completo obrigatório
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "error")
}

func TestLoginHandler_BindJSON_Error(t *testing.T) {
	router := setupUnitRouter()

	// Payload vazio e sem DPoP Header
	payloadInvalido := []byte(`{}`)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/login", bytes.NewBuffer(payloadInvalido))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRefreshHandler_BindJSON_Error(t *testing.T) {
	router := setupUnitRouter()

	payloadInvalido := []byte(`{}`)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/refresh", bytes.NewBuffer(payloadInvalido))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateUserHandler_BindJSON_Error(t *testing.T) {
	router := setupUnitRouter()

	// "name" was expecting a string, not an integer
	payloadInvalido := []byte(`{"name": 123}`)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", "/api/users/1", bytes.NewBuffer(payloadInvalido))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestServicesNil(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.Default()
	userHandler := userHttp.NewUserHandler(nil, nil, nil, nil, nil)
	
	router.POST("/api/signup", userHandler.Signup)
	router.POST("/api/login", userHandler.Login)
	router.POST("/api/refresh", userHandler.Refresh)
	router.PUT("/api/users/:id", userHandler.UpdateUser)
	router.POST("/api/users/:id/send-verify-email", userHandler.SendVerifyEmail)

	tests := []struct {
		Method string
		URL    string
	}{
		{"POST", "/api/signup"},
		{"POST", "/api/login"},
		{"POST", "/api/refresh"},
		{"PUT", "/api/users/1"},
		{"POST", "/api/users/1/send-verify-email"},
	}

	for _, tt := range tests {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(tt.Method, tt.URL, nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	}
}

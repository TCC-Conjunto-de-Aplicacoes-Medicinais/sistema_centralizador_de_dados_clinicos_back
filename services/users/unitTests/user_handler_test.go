package unitTests

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	userHttp "github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/services/users/internal/http"
	
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func setupUnitRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.Default()

	// Injetamos nil nos serviços já que o objetivo aqui é testar
	// estritamente o comportamento do Handlers HTTP (ex: Bindings de JSON).
	userHandler := userHttp.NewUserHandler(nil, nil)

	router.POST("/api/signup", userHandler.Signup)
	router.POST("/api/login", userHandler.Login)
	
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

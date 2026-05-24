package unitTests

import (
	"bytes"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Nerzal/gocloak/v13"
	userHttp "github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/services/users/core/http"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/services/users/core/services"
	sharedConfig "github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/config"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/logger"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	
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
		nil,
		nil,
	)

	// Mock AuthMiddleware for unit tests
	router.Use(func(c *gin.Context) {
		c.Set("userID", "f5c09322-2624-4f0e-b816-7fb4b2b2b2b2")
		c.Next()
	})

	router.POST("/api/signup", userHandler.Signup)
	router.POST("/api/login", userHandler.Login)
	router.POST("/api/refresh", userHandler.Refresh)
	router.PUT("/api/users", userHandler.UpdateUser)
	router.POST("/api/users/send-verify-email", userHandler.SendVerifyEmail)
	router.POST("/api/users/verify-email-code", userHandler.VerifyCode)
	router.POST("/api/users/exams/share", userHandler.ShareExam)
	
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
	req := httptest.NewRequest("PUT", "/api/users", bytes.NewBuffer(payloadInvalido))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestShareExamHandler_BindJSON_Error(t *testing.T) {
	router := setupUnitRouter()

	payloadInvalido := []byte(`{}`)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/users/exams/share", bytes.NewBuffer(payloadInvalido))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestServicesNil(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.Default()
	userHandler := userHttp.NewUserHandler(nil, nil, nil, nil, nil, nil, nil)
	
	router.POST("/api/signup", userHandler.Signup)
	router.POST("/api/login", userHandler.Login)
	router.POST("/api/refresh", userHandler.Refresh)
	router.PUT("/api/users", userHandler.UpdateUser)
	router.POST("/api/users/send-verify-email", userHandler.SendVerifyEmail)
	router.POST("/api/users/verify-email-code", userHandler.VerifyCode)
	router.POST("/api/users/exams/share", userHandler.ShareExam)
	router.GET("/api/users/profile", userHandler.GetUserProfile)

	tests := []struct {
		Method string
		URL    string
	}{
		{"POST", "/api/signup"},
		{"POST", "/api/login"},
		{"POST", "/api/refresh"},
		{"PUT", "/api/users"},
		{"POST", "/api/users/send-verify-email"},
		{"POST", "/api/users/verify-email-code"},
		{"GET", "/api/users/profile"},
	}

	for _, tt := range tests {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(tt.Method, tt.URL, nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	}
}

func TestGetUserProfile_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      db,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	assert.NoError(t, err)

	getUserService := services.NewGetUserService(gormDB)
	userHandler := userHttp.NewUserHandler(nil, nil, nil, nil, getUserService, nil, nil)

	gin.SetMode(gin.TestMode)
	router := gin.Default()
	router.Use(func(c *gin.Context) {
		c.Set("userID", "keycloak-123")
		c.Next()
	})
	router.GET("/api/users/profile", userHandler.GetUserProfile)

	patientUUID := "patient-uuid-123"
	mock.ExpectQuery("SELECT id, name FROM patient WHERE keycloak_id = \\?").
		WithArgs("keycloak-123").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(patientUUID, "Nome do Paciente"))

	mock.ExpectQuery("SELECT phone FROM patient_phone WHERE patient_id = \\? AND principal = true").
		WithArgs(patientUUID).
		WillReturnRows(sqlmock.NewRows([]string{"phone"}).AddRow("11999998888"))

	mock.ExpectQuery("SELECT address FROM patient_address WHERE patient_id = \\? AND principal = true").
		WithArgs(patientUUID).
		WillReturnRows(sqlmock.NewRows([]string{"address"}).AddRow("Rua das Oliveiras, 456"))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/users/profile", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"name":"Nome do Paciente"`)
	assert.Contains(t, w.Body.String(), `"phone":"11999998888"`)
	assert.Contains(t, w.Body.String(), `"address":"Rua das Oliveiras, 456"`)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetUserProfile_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      db,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	assert.NoError(t, err)

	getUserService := services.NewGetUserService(gormDB)
	userHandler := userHttp.NewUserHandler(nil, nil, nil, nil, getUserService, nil, nil)

	gin.SetMode(gin.TestMode)
	router := gin.Default()
	router.Use(func(c *gin.Context) {
		c.Set("userID", "keycloak-123")
		c.Next()
	})
	router.GET("/api/users/profile", userHandler.GetUserProfile)

	mock.ExpectQuery("SELECT id, name FROM patient WHERE keycloak_id = \\?").
		WithArgs("keycloak-123").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/users/profile", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "paciente não encontrado")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetUserProfile_Unauthorized(t *testing.T) {
	getUserService := &services.GetUserService{}
	userHandler := userHttp.NewUserHandler(nil, nil, nil, nil, getUserService, nil, nil)

	gin.SetMode(gin.TestMode)
	router := gin.Default()
	router.GET("/api/users/profile", userHandler.GetUserProfile)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/users/profile", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestShareExam_Success(t *testing.T) {
	appLogger := logger.NewLogger(nil)
	userHandler := userHttp.NewUserHandler(nil, nil, nil, nil, nil, nil, appLogger)

	gin.SetMode(gin.TestMode)
	router := gin.Default()
	router.Use(func(c *gin.Context) {
		c.Set("userID", "f5c09322-2624-4f0e-b816-7fb4b2b2b2b2")
		c.Next()
	})
	router.POST("/api/users/exams/share", userHandler.ShareExam)

	payload := []byte(`{"exam_id": "exam-123", "doctor_name": "Dr. House"}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/users/exams/share", bytes.NewBuffer(payload))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Exame compartilhado com sucesso!")
}

func TestShareExam_Unauthorized(t *testing.T) {
	userHandler := userHttp.NewUserHandler(nil, nil, nil, nil, nil, nil, nil)

	gin.SetMode(gin.TestMode)
	router := gin.Default()
	router.POST("/api/users/exams/share", userHandler.ShareExam)

	payload := []byte(`{"exam_id": "exam-123", "doctor_name": "Dr. House"}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/users/exams/share", bytes.NewBuffer(payload))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func setupHandlerMockKeycloakServer(t *testing.T) (*httptest.Server, *sharedConfig.KeycloakAuth) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "POST" && strings.Contains(r.URL.Path, "/protocol/openid-connect/token") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"access_token": "mock-access-token"}`))
			return
		}
		if r.Method == "GET" && strings.Contains(r.URL.Path, "/users/f5c09322-2624-4f0e-b816-7fb4b2b2b2b2") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id": "f5c09322-2624-4f0e-b816-7fb4b2b2b2b2", "username": "cpf", "email": "test@example.com"}`))
			return
		}
		if r.Method == "PUT" && strings.Contains(r.URL.Path, "/users/f5c09322-2624-4f0e-b816-7fb4b2b2b2b2") {
			w.WriteHeader(http.StatusOK)
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

func TestSendVerifyEmail_Handler_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      db,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	assert.NoError(t, err)

	smtpService := services.NewSMTPEmailService(&sharedConfig.SMTPConfig{})
	verifyService := services.NewVerifyEmailService(gormDB, nil, smtpService)
	appLogger := logger.NewLogger(nil)
	userHandler := userHttp.NewUserHandler(nil, nil, nil, verifyService, nil, nil, appLogger)

	gin.SetMode(gin.TestMode)
	router := gin.Default()
	router.Use(func(c *gin.Context) {
		c.Set("userID", "f5c09322-2624-4f0e-b816-7fb4b2b2b2b2")
		c.Next()
	})
	router.POST("/api/users/send-verify-email", userHandler.SendVerifyEmail)

	mock.ExpectQuery("SELECT id, verify FROM patient WHERE keycloak_id = \\?").
		WithArgs("f5c09322-2624-4f0e-b816-7fb4b2b2b2b2").
		WillReturnRows(sqlmock.NewRows([]string{"id", "verify"}).AddRow("patient-uuid", false))

	mock.ExpectQuery("SELECT email FROM patient_email").
		WithArgs("patient-uuid").
		WillReturnRows(sqlmock.NewRows([]string{"email"}).AddRow("test@example.com"))

	mock.ExpectExec("UPDATE patient SET verification_code = \\?, updated_at = NOW\\(\\) WHERE id = \\?").
		WithArgs(sqlmock.AnyArg(), "patient-uuid").
		WillReturnResult(sqlmock.NewResult(1, 1))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/users/send-verify-email", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
	assert.Contains(t, w.Body.String(), "E-mail de verificação enviado com sucesso")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSendVerifyEmail_Handler_Failure(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      db,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	assert.NoError(t, err)

	verifyService := services.NewVerifyEmailService(gormDB, nil, nil)
	appLogger := logger.NewLogger(nil)
	userHandler := userHttp.NewUserHandler(nil, nil, nil, verifyService, nil, nil, appLogger)

	gin.SetMode(gin.TestMode)
	router := gin.Default()
	router.Use(func(c *gin.Context) {
		c.Set("userID", "f5c09322-2624-4f0e-b816-7fb4b2b2b2b2")
		c.Next()
	})
	router.POST("/api/users/send-verify-email", userHandler.SendVerifyEmail)

	mock.ExpectQuery("SELECT id, verify FROM patient WHERE keycloak_id = \\?").
		WithArgs("f5c09322-2624-4f0e-b816-7fb4b2b2b2b2").
		WillReturnError(sql.ErrNoRows)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/users/send-verify-email", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "paciente não encontrado")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestVerifyCode_Handler_Success(t *testing.T) {
	server, kcAuth := setupHandlerMockKeycloakServer(t)
	defer server.Close()

	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      db,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	assert.NoError(t, err)

	verifyService := services.NewVerifyEmailService(gormDB, kcAuth, nil)
	appLogger := logger.NewLogger(nil)
	userHandler := userHttp.NewUserHandler(nil, nil, nil, verifyService, nil, nil, appLogger)

	gin.SetMode(gin.TestMode)
	router := gin.Default()
	router.Use(func(c *gin.Context) {
		c.Set("userID", "f5c09322-2624-4f0e-b816-7fb4b2b2b2b2")
		c.Next()
	})
	router.POST("/api/users/verify-email-code", userHandler.VerifyCode)

	mock.ExpectQuery("SELECT id, verify, COALESCE\\(verification_code, ''\\) FROM patient WHERE keycloak_id = \\?").
		WithArgs("f5c09322-2624-4f0e-b816-7fb4b2b2b2b2").
		WillReturnRows(sqlmock.NewRows([]string{"id", "verify", "verification_code"}).AddRow("patient-uuid", false, "123456"))

	mock.ExpectExec("UPDATE patient SET verify = true, verification_code = '', updated_at = NOW\\(\\) WHERE id = \\?").
		WithArgs("patient-uuid").
		WillReturnResult(sqlmock.NewResult(1, 1))

	payload := []byte(`{"code": "123456"}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/users/verify-email-code", bytes.NewBuffer(payload))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "E-mail verificado com sucesso")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestVerifyCode_Handler_Failure_InvalidPayload(t *testing.T) {
	appLogger := logger.NewLogger(nil)
	userHandler := userHttp.NewUserHandler(nil, nil, nil, &services.VerifyEmailService{}, nil, nil, appLogger)

	gin.SetMode(gin.TestMode)
	router := gin.Default()
	router.Use(func(c *gin.Context) {
		c.Set("userID", "f5c09322-2624-4f0e-b816-7fb4b2b2b2b2")
		c.Next()
	})
	router.POST("/api/users/verify-email-code", userHandler.VerifyCode)

	payload := []byte(`{"invalid": "payload"}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/users/verify-email-code", bytes.NewBuffer(payload))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}


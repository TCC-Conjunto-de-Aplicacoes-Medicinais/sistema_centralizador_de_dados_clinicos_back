package unitTests

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Nerzal/gocloak/v13"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/services/users/core/services"
	sharedConfig "github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/config"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func setupMockKeycloakServer(t *testing.T) (*httptest.Server, *sharedConfig.KeycloakAuth) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "POST" && strings.Contains(r.URL.Path, "/protocol/openid-connect/token") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"access_token": "mock-access-token"}`))
			return
		}
		if r.Method == "GET" && strings.Contains(r.URL.Path, "/users/keycloak-id") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id": "keycloak-id", "username": "cpf", "email": "test@example.com"}`))
			return
		}
		if r.Method == "PUT" && strings.Contains(r.URL.Path, "/users/keycloak-id") {
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

func TestVerifyEmailService_SendVerificationEmail_PatientNotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      db,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	assert.NoError(t, err)

	smtpService := services.NewSMTPEmailService(&sharedConfig.SMTPConfig{})
	service := services.NewVerifyEmailService(gormDB, nil, smtpService)

	mock.ExpectQuery("SELECT id, verify FROM patient WHERE keycloak_id = \\?").
		WithArgs("keycloak-id").
		WillReturnError(sql.ErrNoRows)

	err = service.SendVerificationEmail("keycloak-id")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "paciente não encontrado no banco de dados")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestVerifyEmailService_SendVerificationEmail_AlreadyVerified(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      db,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	assert.NoError(t, err)

	smtpService := services.NewSMTPEmailService(&sharedConfig.SMTPConfig{})
	service := services.NewVerifyEmailService(gormDB, nil, smtpService)

	mock.ExpectQuery("SELECT id, verify FROM patient WHERE keycloak_id = \\?").
		WithArgs("keycloak-id").
		WillReturnRows(sqlmock.NewRows([]string{"id", "verify"}).AddRow("patient-uuid", true))

	err = service.SendVerificationEmail("keycloak-id")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "e-mail já está verificado")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestVerifyEmailService_SendVerificationEmail_NoEmail(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      db,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	assert.NoError(t, err)

	smtpService := services.NewSMTPEmailService(&sharedConfig.SMTPConfig{})
	service := services.NewVerifyEmailService(gormDB, nil, smtpService)

	mock.ExpectQuery("SELECT id, verify FROM patient WHERE keycloak_id = \\?").
		WithArgs("keycloak-id").
		WillReturnRows(sqlmock.NewRows([]string{"id", "verify"}).AddRow("patient-uuid", false))

	mock.ExpectQuery("SELECT email FROM patient_email").
		WithArgs("patient-uuid").
		WillReturnError(sql.ErrNoRows)

	err = service.SendVerificationEmail("keycloak-id")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nenhum e-mail associado a este paciente")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestVerifyEmailService_SendVerificationEmail_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      db,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	assert.NoError(t, err)

	smtpService := services.NewSMTPEmailService(&sharedConfig.SMTPConfig{})
	service := services.NewVerifyEmailService(gormDB, nil, smtpService)

	mock.ExpectQuery("SELECT id, verify FROM patient WHERE keycloak_id = \\?").
		WithArgs("keycloak-id").
		WillReturnRows(sqlmock.NewRows([]string{"id", "verify"}).AddRow("patient-uuid", false))

	mock.ExpectQuery("SELECT email FROM patient_email").
		WithArgs("patient-uuid").
		WillReturnRows(sqlmock.NewRows([]string{"email"}).AddRow("test@example.com"))

	mock.ExpectExec("UPDATE patient SET verification_code = \\?, updated_at = NOW\\(\\) WHERE id = \\?").
		WithArgs(sqlmock.AnyArg(), "patient-uuid").
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = service.SendVerificationEmail("keycloak-id")
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestVerifyEmailService_VerifyCode_InvalidCode(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      db,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	assert.NoError(t, err)

	service := services.NewVerifyEmailService(gormDB, nil, nil)

	mock.ExpectQuery("SELECT id, verify, COALESCE\\(verification_code, ''\\) FROM patient WHERE keycloak_id = \\?").
		WithArgs("keycloak-id").
		WillReturnRows(sqlmock.NewRows([]string{"id", "verify", "verification_code"}).AddRow("patient-uuid", false, "123456"))

	err = service.VerifyCode("keycloak-id", "654321")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "código inválido")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestVerifyEmailService_VerifyCode_AlreadyVerified(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      db,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	assert.NoError(t, err)

	service := services.NewVerifyEmailService(gormDB, nil, nil)

	mock.ExpectQuery("SELECT id, verify, COALESCE\\(verification_code, ''\\) FROM patient WHERE keycloak_id = \\?").
		WithArgs("keycloak-id").
		WillReturnRows(sqlmock.NewRows([]string{"id", "verify", "verification_code"}).AddRow("patient-uuid", true, "123456"))

	err = service.VerifyCode("keycloak-id", "123456")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "paciente já está verificado")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestVerifyEmailService_VerifyCode_Success(t *testing.T) {
	server, kcAuth := setupMockKeycloakServer(t)
	defer server.Close()

	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      db,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	assert.NoError(t, err)

	service := services.NewVerifyEmailService(gormDB, kcAuth, nil)

	mock.ExpectQuery("SELECT id, verify, COALESCE\\(verification_code, ''\\) FROM patient WHERE keycloak_id = \\?").
		WithArgs("keycloak-id").
		WillReturnRows(sqlmock.NewRows([]string{"id", "verify", "verification_code"}).AddRow("patient-uuid", false, "123456"))

	mock.ExpectExec("UPDATE patient SET verify = true, verification_code = '', updated_at = NOW\\(\\) WHERE id = \\?").
		WithArgs("patient-uuid").
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = service.VerifyCode("keycloak-id", "123456")
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

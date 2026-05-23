package unitTests

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Nerzal/gocloak/v13"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/services/users/core/usecase"
	sharedConfig "github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/config"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/models"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestValidateSignupUseCase_InvalidCPF(t *testing.T) {
	uc := usecase.NewValidateSignupUseCase(nil, nil)
	ctx := context.Background()

	req := models.SignupRequest{
		CPF:   "12345678900", // CPF Inválido
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
		CPF:   "11144477735", // CPF Válido
		Email: "email-invalido",
	}

	err := uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "formato de e-mail inválido")
}

func TestValidateSignupUseCase_DB_CPF_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      db,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	assert.NoError(t, err)

	uc := usecase.NewValidateSignupUseCase(gormDB, nil)
	ctx := context.Background()

	req := models.SignupRequest{
		CPF:   "11144477735",
		Email: "test@example.com",
	}

	mock.ExpectQuery("SELECT count\\(\\*\\) FROM .patient. WHERE cpf = \\?").
		WithArgs("11144477735").
		WillReturnError(errors.New("db error"))

	err = uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "erro ao verificar integridade do CPF no banco de dados")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestValidateSignupUseCase_CPF_AlreadyInUse(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      db,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	assert.NoError(t, err)

	uc := usecase.NewValidateSignupUseCase(gormDB, nil)
	ctx := context.Background()

	req := models.SignupRequest{
		CPF:   "11144477735",
		Email: "test@example.com",
	}

	mock.ExpectQuery("SELECT count\\(\\*\\) FROM .patient. WHERE cpf = \\?").
		WithArgs("11144477735").
		WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(1))

	err = uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "o CPF informado já está em uso")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestValidateSignupUseCase_DB_Email_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      db,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	assert.NoError(t, err)

	uc := usecase.NewValidateSignupUseCase(gormDB, nil)
	ctx := context.Background()

	req := models.SignupRequest{
		CPF:   "11144477735",
		Email: "test@example.com",
	}

	mock.ExpectQuery("SELECT count\\(\\*\\) FROM .patient. WHERE cpf = \\?").
		WithArgs("11144477735").
		WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(0))

	mock.ExpectQuery("SELECT count\\(\\*\\) FROM .patient_email. WHERE email = \\?").
		WithArgs("test@example.com").
		WillReturnError(errors.New("db error"))

	err = uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "erro ao verificar integridade do E-mail no banco de dados")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestValidateSignupUseCase_Email_AlreadyInUse(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      db,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	assert.NoError(t, err)

	uc := usecase.NewValidateSignupUseCase(gormDB, nil)
	ctx := context.Background()

	req := models.SignupRequest{
		CPF:   "11144477735",
		Email: "test@example.com",
	}

	mock.ExpectQuery("SELECT count\\(\\*\\) FROM .patient. WHERE cpf = \\?").
		WithArgs("11144477735").
		WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(0))

	mock.ExpectQuery("SELECT count\\(\\*\\) FROM .patient_email. WHERE email = \\?").
		WithArgs("test@example.com").
		WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(1))

	err = uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "o E-mail informado já está em uso")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestValidateSignupUseCase_Success_Keycloak_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      db,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	assert.NoError(t, err)

	kcClient := gocloak.NewClient("http://localhost:12345") // Fake server
	kcAuth := &sharedConfig.KeycloakAuth{
		Client:       kcClient,
		ClientID:     "client-id",
		ClientSecret: "secret",
		Realm:        "realm",
	}

	uc := usecase.NewValidateSignupUseCase(gormDB, kcAuth)
	ctx := context.Background()

	req := models.SignupRequest{
		CPF:   "11144477735",
		Email: "test@example.com",
	}

	mock.ExpectQuery("SELECT count\\(\\*\\) FROM .patient. WHERE cpf = \\?").
		WithArgs("11144477735").
		WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(0))

	mock.ExpectQuery("SELECT count\\(\\*\\) FROM .patient_email. WHERE email = \\?").
		WithArgs("test@example.com").
		WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(0))

	// Should not panic, skip Keycloak check block on error, and return nil
	err = uc.Execute(ctx, req)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

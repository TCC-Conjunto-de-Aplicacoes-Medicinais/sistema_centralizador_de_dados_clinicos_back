package unitTests

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/services/users/core/services"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestGetUserService_GetProfile_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      db,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	assert.NoError(t, err)

	service := services.NewGetUserService(gormDB)

	keycloakID := "keycloak-123"
	patientUUID := "patient-uuid-123"

	// 1. Mock select patient
	mock.ExpectQuery("SELECT id, name FROM patient WHERE keycloak_id = \\?").
		WithArgs(keycloakID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(patientUUID, "Nome do Paciente"))

	// 2. Mock select phone
	mock.ExpectQuery("SELECT phone FROM patient_phone WHERE patient_id = \\? AND principal = true").
		WithArgs(patientUUID).
		WillReturnRows(sqlmock.NewRows([]string{"phone"}).AddRow("11999998888"))

	// 3. Mock select address
	mock.ExpectQuery("SELECT address FROM patient_address WHERE patient_id = \\? AND principal = true").
		WithArgs(patientUUID).
		WillReturnRows(sqlmock.NewRows([]string{"address"}).AddRow("Rua das Oliveiras, 456"))

	profile, err := service.GetProfile(keycloakID)
	assert.NoError(t, err)
	assert.NotNil(t, profile)
	assert.Equal(t, "Nome do Paciente", profile.Name)
	assert.Equal(t, "11999998888", profile.Phone)
	assert.Equal(t, "Rua das Oliveiras, 456", profile.Address)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetUserService_GetProfile_NoPhoneAddress(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      db,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	assert.NoError(t, err)

	service := services.NewGetUserService(gormDB)

	keycloakID := "keycloak-123"
	patientUUID := "patient-uuid-123"

	// 1. Mock select patient
	mock.ExpectQuery("SELECT id, name FROM patient WHERE keycloak_id = \\?").
		WithArgs(keycloakID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(patientUUID, "Nome do Paciente"))

	// 2. Mock select phone (no rows)
	mock.ExpectQuery("SELECT phone FROM patient_phone WHERE patient_id = \\? AND principal = true").
		WithArgs(patientUUID).
		WillReturnRows(sqlmock.NewRows([]string{"phone"}))

	// 3. Mock select address (no rows)
	mock.ExpectQuery("SELECT address FROM patient_address WHERE patient_id = \\? AND principal = true").
		WithArgs(patientUUID).
		WillReturnRows(sqlmock.NewRows([]string{"address"}))

	profile, err := service.GetProfile(keycloakID)
	assert.NoError(t, err)
	assert.NotNil(t, profile)
	assert.Equal(t, "Nome do Paciente", profile.Name)
	assert.Equal(t, "", profile.Phone)
	assert.Equal(t, "", profile.Address)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetUserService_GetProfile_PatientNotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      db,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	assert.NoError(t, err)

	service := services.NewGetUserService(gormDB)

	keycloakID := "keycloak-nonexistent"

	// 1. Mock select patient (no rows)
	mock.ExpectQuery("SELECT id, name FROM patient WHERE keycloak_id = \\?").
		WithArgs(keycloakID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}))

	profile, err := service.GetProfile(keycloakID)
	assert.Error(t, err)
	assert.Nil(t, profile)
	assert.Equal(t, "paciente não encontrado", err.Error())

	assert.NoError(t, mock.ExpectationsWereMet())
}

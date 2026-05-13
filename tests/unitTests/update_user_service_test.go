package unitTests

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/services/users/core/services"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/models"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestUpdateUserService_UpdateUser_DBSave(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      db,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	assert.NoError(t, err)

	service := services.NewUpdateUserService(gormDB, nil, nil)

	id := "keycloak-123"
	req := models.UpdateUserRequest{
		Name: "Novo Nome",
		Emails: []models.EmailRequest{
			{Email: "novo@teste.com", Principal: true},
		},
		Phones: []models.PhoneRequest{
			{Phone: "11999998888", Principal: true},
		},
		Addresses: []models.AddressRequest{
			{Address: "Rua Teste, 123", Principal: true},
		},
	}

	// 1. Mock select patient
	mock.ExpectQuery("SELECT \\* FROM `patients` WHERE keycloak_id = \\?").
		WithArgs(id, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "keycloak_id"}).AddRow(1, id))

	// 2. Transaction Start
	mock.ExpectBegin()

	// 3. Update patient name
	mock.ExpectExec("UPDATE `patients`").
		WithArgs("Novo Nome", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// 4. Soft Delete old emails
	mock.ExpectExec("UPDATE `patient_emails` SET `deleted_at`=\\?").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// 5. Insert new email
	mock.ExpectExec("INSERT INTO `patient_emails`").
		WillReturnResult(sqlmock.NewResult(1, 1))

	// 6. Soft Delete old phones
	mock.ExpectExec("UPDATE `patient_phones` SET `deleted_at`=\\?").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 0))

	// 6b. Insert new phone
	mock.ExpectExec("INSERT INTO `patient_phones`").
		WillReturnResult(sqlmock.NewResult(1, 1))

	// 7. Soft Delete old addresses
	mock.ExpectExec("UPDATE `patient_addresses` SET `deleted_at`=\\?").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 0))

	// 7b. Insert new address
	mock.ExpectExec("INSERT INTO `patient_addresses`").
		WillReturnResult(sqlmock.NewResult(1, 1))

	// 8. Transaction Commit
	mock.ExpectCommit()

	err = service.UpdateUser(id, req)
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

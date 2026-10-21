package unitTests

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	userHttp "github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/services/users/core/http"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/services/users/core/services"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestRedeemAccessCode_UnknownCPFIsRejected(t *testing.T) {
	db, mock := mockGorm(t)
	mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"id"}))
	codes := services.NewAccessCodeService(db, services.NewClinicalSyncService(db, nil))

	_, _, err := codes.Redeem(t.Context(), "52998224725", "123456")
	assert.ErrorIs(t, err, services.ErrAccessCodeInvalid)
}

func TestRedeemAccessCode_RejectsMalformedBody(t *testing.T) {
	h := userHttp.NewAccessCodeHandler(nil, nil, nil, nil)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/access-codes/redeem", h.Redeem)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, jsonRequest(http.MethodPost, "/api/access-codes/redeem", `{"cpf":"52998224725","code":"12"}`))
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGenerateAccessCode_RequiresPasskeyAssertion(t *testing.T) {
	db, mock := mockGorm(t)
	mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("paciente-1"))
	sync := services.NewClinicalSyncService(db, nil)
	h := userHttp.NewAccessCodeHandler(services.NewAccessCodeService(db, sync), sync, newPasskeyService(t, db), nil)
	r := patientRouter(http.MethodPost, "/api/access-codes", h.Generate)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, jsonRequest(http.MethodPost, "/api/access-codes", ""))
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestGenerateAccessCode_UnknownPatientIsNotFound(t *testing.T) {
	db, mock := mockGorm(t)
	mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"id"}))
	sync := services.NewClinicalSyncService(db, nil)
	h := userHttp.NewAccessCodeHandler(services.NewAccessCodeService(db, sync), sync, newPasskeyService(t, db), nil)
	r := patientRouter(http.MethodPost, "/api/access-codes", h.Generate)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, jsonRequest(http.MethodPost, "/api/access-codes", ""))
	assert.Equal(t, http.StatusNotFound, w.Code)
}

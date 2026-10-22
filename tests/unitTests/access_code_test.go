package unitTests

import (
	"errors"
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

	_, _, err := codes.Redeem(t.Context(), "52998224725", "123456", func(string) error { return nil })
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

func TestBreakGlass_RequiresJustification(t *testing.T) {
	h := userHttp.NewClinicHandler(nil, nil, nil)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/patients/:id/break-glass", h.BreakGlass)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, jsonRequest(http.MethodPost, "/api/patients/paciente-1/break-glass", `{"justificativa":"curto"}`))
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestIsDemoPatient_OnlyExampleDomain(t *testing.T) {
	db, mock := mockGorm(t)
	mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	sync := services.NewClinicalSyncService(db, nil)

	demo, err := sync.IsDemoPatient(t.Context(), "paciente-real")
	assert.NoError(t, err)
	assert.False(t, demo)
}

func TestClinicSync_RejectsInvalidCPF(t *testing.T) {
	h := userHttp.NewClinicHandler(services.NewClinicalSyncService(nil, nil), nil, nil)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/patients/sync", h.SyncPatient)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, jsonRequest(http.MethodPost, "/api/patients/sync", `{"name":"Ana","cpf":"123","birth_date":"1990-01-01"}`))
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGrantConsent_RejectsMalformedBody(t *testing.T) {
	db, mock := mockGorm(t)
	mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("paciente-1"))
	sync := services.NewClinicalSyncService(db, nil)
	h := userHttp.NewConsentHandler(sync, nil, newPasskeyService(t, db))
	r := patientRouter(http.MethodPost, "/api/consents", h.GrantConsent)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, jsonRequest(http.MethodPost, "/api/consents", `{"scope":`))
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListConsents_DatabaseErrorIsInternal(t *testing.T) {
	db, mock := mockGorm(t)
	mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("paciente-1"))
	mock.ExpectQuery("SELECT").WillReturnError(errors.New("banco indisponível"))
	sync := services.NewClinicalSyncService(db, nil)
	h := userHttp.NewConsentHandler(sync, nil, nil)
	r := patientRouter(http.MethodGet, "/api/consents", h.ListConsents)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, jsonRequest(http.MethodGet, "/api/consents", ""))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

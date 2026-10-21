package unitTests

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Nerzal/gocloak/v13"
	userHttp "github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/services/users/core/http"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/services/users/core/services"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// mockGorm abre o GORM sobre um sqlmock, para cobrir os caminhos que consultam o banco.
func mockGorm(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	return db, mock
}

func newPasskeyService(t *testing.T, db *gorm.DB) *services.PasskeyService {
	t.Helper()
	svc, err := services.NewPasskeyService(db, nil, "localhost", []string{"http://localhost:8081"})
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

func jsonRequest(method, path, body string) *http.Request {
	req, _ := http.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

// patientRouter simula o AuthMiddleware: injeta o userID (sub do Keycloak) no contexto.
func patientRouter(method, path string, handler gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", "kc-paciente-1")
		c.Next()
	})
	r.Handle(method, path, handler)
	return r
}

func TestPasskeyRecoveryRequest_RejectsInvalidEmail(t *testing.T) {
	h := userHttp.NewPasskeyHandler(nil, nil)
	r := patientRouter(http.MethodPost, "/api/passkeys/recovery/request", h.RequestRecovery)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, jsonRequest(http.MethodPost, "/api/passkeys/recovery/request", `{"email":"nao-e-email"}`))
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPasskeyRecoveryVerify_RejectsMalformedCode(t *testing.T) {
	h := userHttp.NewPasskeyHandler(nil, nil)
	r := patientRouter(http.MethodPost, "/api/passkeys/recovery/verify", h.VerifyRecovery)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, jsonRequest(http.MethodPost, "/api/passkeys/recovery/verify", `{"email":"a@example.com","code":"12"}`))
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestBeginRecoveryRegister_RejectsUnknownToken(t *testing.T) {
	db, mock := mockGorm(t)
	mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"patient_id"}))
	h := userHttp.NewPasskeyHandler(newPasskeyService(t, db), nil)
	r := patientRouter(http.MethodPost, "/api/passkeys/recovery/register/begin", h.BeginRecoveryRegister)

	req := jsonRequest(http.MethodPost, "/api/passkeys/recovery/register/begin", "")
	req.Header.Set("X-Recovery-Token", "token-inexistente")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestGrantConsent_RequiresPasskeyAssertion(t *testing.T) {
	db, mock := mockGorm(t)
	mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("paciente-1"))
	sync := services.NewClinicalSyncService(db, nil)
	h := userHttp.NewConsentHandler(sync, nil, newPasskeyService(t, db))
	r := patientRouter(http.MethodPost, "/api/consents", h.GrantConsent)

	// Sem os headers X-Passkey-Challenge e X-Passkey-Assertion, o consentimento não é gravado.
	w := httptest.NewRecorder()
	r.ServeHTTP(w, jsonRequest(http.MethodPost, "/api/consents", `{"scope":"clinical_history_read"}`))
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRevokeConsent_RequiresPasskeyAssertion(t *testing.T) {
	db, mock := mockGorm(t)
	mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("paciente-1"))
	sync := services.NewClinicalSyncService(db, nil)
	h := userHttp.NewConsentHandler(sync, nil, newPasskeyService(t, db))
	r := patientRouter(http.MethodDelete, "/api/consents/:id", h.RevokeConsent)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, jsonRequest(http.MethodDelete, "/api/consents/consent-1", ""))
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestClinicLogin_LocksAfterThreeFailures(t *testing.T) {
	// Keycloak simulado: recusa qualquer senha.
	kcServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer kcServer.Close()
	kc := &config.KeycloakAuth{Client: gocloak.NewClient(kcServer.URL), ClientID: "c", ClientSecret: "s", Realm: "r"}
	h := userHttp.NewClinicHandler(nil, kc, nil)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/clinic/login", h.Login)

	body := `{"email":"clinica@example.com","password":"errada"}`
	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, jsonRequest(http.MethodPost, "/api/clinic/login", body))
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, jsonRequest(http.MethodPost, "/api/clinic/login", body))
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.NotEmpty(t, w.Header().Get("Retry-After"))
}

func TestClinicSync_RejectsEmptyPayload(t *testing.T) {
	h := userHttp.NewClinicHandler(nil, nil, nil)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/patients/sync", h.SyncPatient)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, jsonRequest(http.MethodPost, "/api/patients/sync", `{"name":`))
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListConsents_ReturnsPatientConsents(t *testing.T) {
	db, mock := mockGorm(t)
	mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("paciente-1"))
	mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"id", "patient_id", "scope"}).AddRow("c-1", "paciente-1", "clinical_history_read"))
	sync := services.NewClinicalSyncService(db, nil)
	h := userHttp.NewConsentHandler(sync, nil, nil)
	r := patientRouter(http.MethodGet, "/api/consents", h.ListConsents)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, jsonRequest(http.MethodGet, "/api/consents", ""))
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "clinical_history_read")
}

func TestIngestRecord_RejectsNonFHIRPayload(t *testing.T) {
	h := userHttp.NewClinicHandler(services.NewClinicalSyncService(nil, nil), nil, nil)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/exams/ingest", h.IngestRecord)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, jsonRequest(http.MethodPost, "/api/exams/ingest", `{"resourceType":"Observation"}`))
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// pacienteComPasskeys simula o banco: o paciente existe e, se pedido, não tem passkeys cadastradas.
func expectPatientWithoutPasskeys(mock sqlmock.Sqlmock) {
	mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("paciente-1"))
	mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"id"}))
}

func TestBeginRegister_ReturnsCreationOptions(t *testing.T) {
	db, mock := mockGorm(t)
	expectPatientWithoutPasskeys(mock)
	h := userHttp.NewPasskeyHandler(newPasskeyService(t, db), services.NewClinicalSyncService(db, nil))
	r := patientRouter(http.MethodPost, "/api/passkeys/register/begin", h.BeginRegister)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, jsonRequest(http.MethodPost, "/api/passkeys/register/begin", ""))
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "challenge_id")
	assert.Contains(t, w.Body.String(), "publicKey")
}

func TestFinishRegister_RejectsMissingChallenge(t *testing.T) {
	db, mock := mockGorm(t)
	mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("paciente-1"))
	h := userHttp.NewPasskeyHandler(newPasskeyService(t, db), services.NewClinicalSyncService(db, nil))
	r := patientRouter(http.MethodPost, "/api/passkeys/register/finish", h.FinishRegister)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, jsonRequest(http.MethodPost, "/api/passkeys/register/finish", `{}`))
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestBeginAction_WithoutPasskeyIsForbidden(t *testing.T) {
	db, mock := mockGorm(t)
	expectPatientWithoutPasskeys(mock)
	h := userHttp.NewPasskeyHandler(newPasskeyService(t, db), services.NewClinicalSyncService(db, nil))
	r := patientRouter(http.MethodPost, "/api/passkeys/action/begin", h.BeginAction)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, jsonRequest(http.MethodPost, "/api/passkeys/action/begin", `{"action":"consent:grant:clinical_history_read"}`))
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestRequestRecovery_UnknownEmailStillAccepted(t *testing.T) {
	db, mock := mockGorm(t)
	mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"patient_id"}))
	h := userHttp.NewPasskeyHandler(newPasskeyService(t, db), nil)
	r := patientRouter(http.MethodPost, "/api/passkeys/recovery/request", h.RequestRecovery)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, jsonRequest(http.MethodPost, "/api/passkeys/recovery/request", `{"email":"ninguem@example.com"}`))
	assert.Equal(t, http.StatusAccepted, w.Code)
}

func TestVerifyRecovery_UnknownEmailIsUnauthorized(t *testing.T) {
	db, mock := mockGorm(t)
	mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"patient_id"}))
	h := userHttp.NewPasskeyHandler(newPasskeyService(t, db), nil)
	r := patientRouter(http.MethodPost, "/api/passkeys/recovery/verify", h.VerifyRecovery)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, jsonRequest(http.MethodPost, "/api/passkeys/recovery/verify", `{"email":"ninguem@example.com","code":"123456"}`))
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestFinishRecoveryRegister_RejectsUnknownToken(t *testing.T) {
	db, mock := mockGorm(t)
	mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"patient_id"}))
	h := userHttp.NewPasskeyHandler(newPasskeyService(t, db), nil)
	r := patientRouter(http.MethodPost, "/api/passkeys/recovery/register/finish", h.FinishRecoveryRegister)

	req := jsonRequest(http.MethodPost, "/api/passkeys/recovery/register/finish", `{}`)
	req.Header.Set("X-Recovery-Token", "token-inexistente")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSearchPatients_RequiresCPFOrName(t *testing.T) {
	db, _ := mockGorm(t)
	h := userHttp.NewClinicHandler(services.NewClinicalSyncService(db, nil), nil, nil)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/patients/search", h.SearchPatients)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, jsonRequest(http.MethodGet, "/api/patients/search", ""))
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

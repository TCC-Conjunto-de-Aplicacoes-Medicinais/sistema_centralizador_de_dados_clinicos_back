package unitTests

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	userHttp "github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/services/users/core/http"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/services/users/core/services"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func clinicRouter(expectedKey string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	g := r.Group("/api")
	g.Use(userHttp.ClinicKeyMiddleware(expectedKey))
	g.GET("/patients/search", func(c *gin.Context) { c.Status(http.StatusOK) })
	return r
}

func TestClinicKeyMiddleware_FailsClosedWithoutServerKey(t *testing.T) {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/patients/search", nil)
	req.Header.Set("X-Clinic-Key", "")
	clinicRouter("").ServeHTTP(w, req)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestClinicKeyMiddleware_RejectsWrongKey(t *testing.T) {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/patients/search", nil)
	req.Header.Set("X-Clinic-Key", "errada")
	clinicRouter("chave-correta").ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestClinicKeyMiddleware_RejectsMissingKey(t *testing.T) {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/patients/search", nil)
	clinicRouter("chave-correta").ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestClinicKeyMiddleware_AcceptsCorrectKey(t *testing.T) {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/patients/search", nil)
	req.Header.Set("X-Clinic-Key", "chave-correta")
	clinicRouter("chave-correta").ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// Validações que acontecem antes de qualquer acesso ao banco (DB nil não é usado).
func TestIngestDiagnosticReport_RejectsInvalidPayloads(t *testing.T) {
	svc := services.NewClinicalSyncService(nil, nil)
	ctx := context.Background()

	_, err := svc.IngestDiagnosticReport(ctx, map[string]interface{}{"resourceType": "Observation"})
	assert.ErrorContains(t, err, "DiagnosticReport")

	_, err = svc.IngestDiagnosticReport(ctx, map[string]interface{}{
		"resourceType": "DiagnosticReport",
		"id":           "rep-1",
	})
	assert.ErrorContains(t, err, "meta.clinic_id")
}

func TestUpsertPatient_RejectsInvalidCPFAndBirthDate(t *testing.T) {
	svc := services.NewClinicalSyncService(nil, nil)
	ctx := context.Background()

	_, err := svc.UpsertPatient(ctx, services.PatientSyncInput{Name: "Ana", CPF: "123", BirthDate: "1990-01-01"})
	assert.ErrorContains(t, err, "CPF inválido")

	_, err = svc.UpsertPatient(ctx, services.PatientSyncInput{Name: "Ana", CPF: "12345678901", BirthDate: "ontem"})
	assert.ErrorContains(t, err, "data de nascimento inválida")
}

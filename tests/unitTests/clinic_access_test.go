package unitTests

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	userHttp "github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/services/users/core/http"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// verifierFor simula a introspecção do Keycloak: devolve as claims do token informado.
func verifierFor(claims map[string]interface{}, valid bool) userHttp.ClaimsVerifier {
	return func(_ context.Context, _ string) (map[string]interface{}, error) {
		if !valid {
			return nil, errors.New("token inativo, expirado ou revogado")
		}
		return claims, nil
	}
}

func accessRouter(key string, verify userHttp.ClaimsVerifier) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	g := r.Group("/api")
	g.Use(userHttp.ClinicAccessMiddleware(key, verify))
	g.GET("/patients/search", func(c *gin.Context) { c.Status(http.StatusOK) })
	return r
}

func doAccess(r *gin.Engine, headers map[string]string) int {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/patients/search", nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	r.ServeHTTP(w, req)
	return w.Code
}

func TestClinicAccess_IntegrationKeyAccepted(t *testing.T) {
	r := accessRouter("chave-correta", verifierFor(nil, false))
	assert.Equal(t, http.StatusOK, doAccess(r, map[string]string{"X-Clinic-Key": "chave-correta"}))
}

func TestClinicAccess_ClinicUserTokenAccepted(t *testing.T) {
	r := accessRouter("chave-correta", verifierFor(map[string]interface{}{"sub": "u1", "tipo_usuario": "clinica"}, true))
	assert.Equal(t, http.StatusOK, doAccess(r, map[string]string{"Authorization": "Bearer abc"}))
}

func TestClinicAccess_ClinicAttributeAsListAccepted(t *testing.T) {
	r := accessRouter("", verifierFor(map[string]interface{}{"sub": "u1", "tipo_usuario": []interface{}{"clinica"}}, true))
	assert.Equal(t, http.StatusOK, doAccess(r, map[string]string{"Authorization": "Bearer abc"}))
}

func TestClinicAccess_PatientTokenForbidden(t *testing.T) {
	// Conta de paciente (sem tipo_usuario=clinica) não pode buscar pacientes.
	r := accessRouter("chave-correta", verifierFor(map[string]interface{}{"sub": "p1"}, true))
	assert.Equal(t, http.StatusForbidden, doAccess(r, map[string]string{"Authorization": "Bearer abc"}))
}

func TestClinicAccess_InvalidTokenUnauthorized(t *testing.T) {
	r := accessRouter("chave-correta", verifierFor(nil, false))
	assert.Equal(t, http.StatusUnauthorized, doAccess(r, map[string]string{"Authorization": "Bearer abc"}))
}

func TestClinicAccess_WrongKeyAndNoTokenUnauthorized(t *testing.T) {
	r := accessRouter("chave-correta", verifierFor(map[string]interface{}{"tipo_usuario": "clinica"}, true))
	assert.Equal(t, http.StatusUnauthorized, doAccess(r, map[string]string{"X-Clinic-Key": "errada"}))
	assert.Equal(t, http.StatusUnauthorized, doAccess(r, nil))
}

func TestClinicAccess_NoServerKeyDoesNotAcceptEmptyKey(t *testing.T) {
	// Sem chave configurada, um X-Clinic-Key vazio não pode liberar acesso.
	r := accessRouter("", verifierFor(nil, false))
	assert.Equal(t, http.StatusUnauthorized, doAccess(r, map[string]string{"X-Clinic-Key": ""}))
}

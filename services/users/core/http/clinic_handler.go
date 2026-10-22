package http

import (
	"context"
	"crypto/subtle"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/services/users/core/services"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/config"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/database"
	"github.com/gin-gonic/gin"
)

// ClaimsVerifier valida o access token e devolve as claims (ver KeycloakAuth.VerifyAccessToken).
type ClaimsVerifier func(ctx context.Context, token string) (map[string]interface{}, error)

// isClinicClaims indica se a conta é de clínica: atributo tipo_usuario = "clinica",
// gravado pelo Wizard no cadastro e exposto no token por um mapper do client.
func isClinicClaims(claims map[string]interface{}) bool {
	switch v := claims["tipo_usuario"].(type) {
	case string:
		return v == "clinica"
	case []interface{}:
		for _, item := range v {
			if s, ok := item.(string); ok && s == "clinica" {
				return true
			}
		}
	}
	return false
}

// ClinicKeyMiddleware autentica só pela chave de integração (X-Clinic-Key).
// Sem chave configurada no servidor, rejeita tudo (fail closed).
func ClinicKeyMiddleware(expected string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if expected == "" {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "integração com clínicas não configurada"})
			return
		}
		got := c.GetHeader("X-Clinic-Key")
		if subtle.ConstantTimeCompare([]byte(got), []byte(expected)) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "chave de clínica inválida"})
			return
		}
		c.Set("clinicActor", "integracao")
		c.Next()
	}
}

// ClinicAccessMiddleware aceita duas formas de acesso às rotas da clínica:
//   - X-Clinic-Key: integração entre sistemas (ex.: template da clínica).
//   - Authorization: Bearer <token>: usuário de clínica logado no front (conta tipo_usuario=clinica).
//
// O front roda no navegador, então não pode carregar a chave de integração.
func ClinicAccessMiddleware(expectedKey string, verify ClaimsVerifier) gin.HandlerFunc {
	return func(c *gin.Context) {
		if got := c.GetHeader("X-Clinic-Key"); expectedKey != "" && got != "" &&
			subtle.ConstantTimeCompare([]byte(got), []byte(expectedKey)) == 1 {
			c.Set("clinicActor", "integracao")
			c.Next()
			return
		}

		parts := strings.SplitN(c.GetHeader("Authorization"), " ", 2)
		if len(parts) == 2 && parts[0] == "Bearer" && verify != nil {
			claims, err := verify(c.Request.Context(), parts[1])
			if err != nil {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token inválido: " + err.Error()})
				return
			}
			if !isClinicClaims(claims) {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "conta não habilitada para clínica"})
				return
			}
			// Ator da auditoria: e-mail da conta da clínica (legível); o sub é o fallback.
			actor, _ := claims["email"].(string)
			if actor == "" {
				actor, _ = claims["sub"].(string)
			}
			c.Set("clinicActor", actor)
			c.Next()
			return
		}

		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "credenciais de clínica ausentes"})
	}
}

// ClinicHandler expõe ao centralizador o contrato das clínicas (integração) e ao front da clínica (login e leitura).
type ClinicHandler struct {
	sync  *services.ClinicalSyncService
	kc    *config.KeycloakAuth
	audit   *services.AccessAuditService
	limiter *loginLimiter
}

// Limite de login da clínica: 3 senhas erradas seguidas bloqueiam o e-mail por 15 minutos.
func NewClinicHandler(sync *services.ClinicalSyncService, kc *config.KeycloakAuth, audit *services.AccessAuditService) *ClinicHandler {
	return &ClinicHandler{sync: sync, kc: kc, audit: audit, limiter: newLoginLimiter(3, 15*time.Minute)}
}

type clinicLoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// Login POST /api/clinic/login — login do usuário da clínica. Só contas tipo_usuario=clinica recebem token.
func (h *ClinicHandler) Login(c *gin.Context) {
	var req clinicLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "e-mail e senha são obrigatórios"})
		return
	}
	now := time.Now()
	if wait := h.limiter.blockedFor(req.Email, now); wait > 0 {
		c.Header("Retry-After", strconv.Itoa(int(wait.Seconds())+1))
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "muitas tentativas; aguarde antes de tentar de novo"})
		return
	}

	ctx := c.Request.Context()
	tok, err := h.kc.Client.Login(ctx, h.kc.ClientID, h.kc.ClientSecret, h.kc.Realm, req.Email, req.Password)
	if err != nil {
		// Só o motivo devolvido pelo Keycloak vai para o log; a senha nunca é registrada.
		log.Printf("[clinic/login] Keycloak recusou o login (realm=%s client=%s): %v", h.kc.Realm, h.kc.ClientID, err)
		h.limiter.recordFailure(req.Email, now)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "credenciais inválidas"})
		return
	}
	claims, err := h.kc.VerifyAccessToken(ctx, tok.AccessToken)
	if err != nil || !isClinicClaims(claims) {
		c.JSON(http.StatusForbidden, gin.H{"error": "conta não habilitada para clínica"})
		return
	}
	h.limiter.reset(req.Email)
	// O Wizard grava o nome da clínica como primeiro nome da conta (given_name).
	name, _ := claims["name"].(string)
	clinicName, _ := claims["given_name"].(string)
	sub, _ := claims["sub"].(string)
	c.JSON(http.StatusOK, gin.H{
		"access_token":  tok.AccessToken,
		"refresh_token": tok.RefreshToken,
		"expires_in":    tok.ExpiresIn,
		"user":          gin.H{"name": name, "email": req.Email, "role": "Profissional Clínico"},
		"clinic":        gin.H{"id": sub, "name": clinicName, "type": "partner"},
	})
}

type patientSummary struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CPF       string `json:"cpf"`
	BirthDate string `json:"birthDate"`
}

func summarize(p database.Patient) patientSummary {
	return patientSummary{
		ID:        p.Id,
		Name:      p.Name,
		CPF:       p.CPF,
		BirthDate: p.BirthDate.Format("2006-01-02"),
	}
}

// SearchPatients GET /api/patients/search?cpf=...|name=... (auditado)
func (h *ClinicHandler) SearchPatients(c *gin.Context) {
	list, err := h.sync.SearchPatients(c.Request.Context(), c.Query("cpf"), c.Query("name"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.audit.Record(c.GetString("clinicActor"), "search", "", "ok"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "não foi possível registrar a auditoria"})
		return
	}
	out := make([]patientSummary, 0, len(list))
	for _, p := range list {
		out = append(out, summarize(p))
	}
	resp := gin.H{"found": len(out) > 0, "patients": out}
	if len(out) > 0 {
		resp["patient"] = out[0]
	}
	c.JSON(http.StatusOK, resp)
}

type patientSyncRequest struct {
	ID        string `json:"id"` // identificador local da clínica; o centralizador usa o CPF como chave
	Name      string `json:"name"`
	CPF       string `json:"cpf"`
	BirthDate string `json:"birth_date"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
	Gender    string `json:"gender"`
}

// SyncPatient POST /api/patients/sync — cria ou atualiza o paciente pelo CPF.
func (h *ClinicHandler) SyncPatient(c *gin.Context) {
	var req patientSyncRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "payload inválido: " + err.Error()})
		return
	}
	p, err := h.sync.UpsertPatient(c.Request.Context(), services.PatientSyncInput{
		Name:      req.Name,
		CPF:       req.CPF,
		BirthDate: req.BirthDate,
		Email:     req.Email,
		Phone:     req.Phone,
		Gender:    req.Gender,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"found": true, "id": p.Id})
}

// IngestRecord POST /api/exams/ingest — recebe um DiagnosticReport FHIR R4 de uma clínica.
func (h *ClinicHandler) IngestRecord(c *gin.Context) {
	var report map[string]interface{}
	if err := c.ShouldBindJSON(&report); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "payload FHIR inválido: " + err.Error()})
		return
	}
	res, err := h.sync.IngestDiagnosticReport(c.Request.Context(), report)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"status": "ingested", "patient_id": res.PatientID, "record_id": res.RecordID})
}

// PatientAudit GET /api/patients/:id/audit — acessos aos dados do paciente (tela de auditoria LGPD da clínica).
func (h *ClinicHandler) PatientAudit(c *gin.Context) {
	entries, err := h.audit.ListByPatient(c.Param("id"), 100)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "falha ao consultar auditoria"})
		return
	}
	c.JSON(http.StatusOK, entries)
}

// FHIRBundle GET /api/patients/:id/fhir-bundle — histórico federado, só com consentimento vigente.
// Toda tentativa (permitida ou negada) é auditada; se a auditoria falhar, nenhum dado sai.
func (h *ClinicHandler) FHIRBundle(c *gin.Context) {
	patientID := c.Param("id")
	actor := c.GetString("clinicActor")

	bundle, err := h.sync.BuildPatientBundle(c.Request.Context(), patientID)
	result := "ok"
	switch {
	case errors.Is(err, services.ErrNoConsent):
		result = "sem_consentimento"
	case errors.Is(err, services.ErrPatientNotFound):
		result = "nao_encontrado"
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "falha ao montar bundle FHIR"})
		return
	}

	if auditErr := h.audit.Record(actor, "read_bundle", patientID, result); auditErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "não foi possível registrar a auditoria"})
		return
	}

	switch result {
	case "sem_consentimento":
		c.JSON(http.StatusForbidden, gin.H{"error": "consent_required"})
	case "nao_encontrado":
		c.JSON(http.StatusNotFound, gin.H{"error": "paciente não encontrado"})
	default:
		c.JSON(http.StatusOK, bundle)
	}
}

package http

import (
	"bytes"
	"encoding/base64"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/services/users/core/services"
	"github.com/gin-gonic/gin"
)

// ConsentHandler permite ao paciente autenticado (DPoP + token) gerenciar seus consentimentos
// e consultar quem acessou os seus dados. Conceder e revogar exigem a assinatura da passkey.
type ConsentHandler struct {
	sync     *services.ClinicalSyncService
	audit    *services.AccessAuditService
	passkeys *services.PasskeyService
}

func NewConsentHandler(sync *services.ClinicalSyncService, audit *services.AccessAuditService, passkeys *services.PasskeyService) *ConsentHandler {
	return &ConsentHandler{sync: sync, audit: audit, passkeys: passkeys}
}

// verifyPasskey confere a assinatura da passkey para a ação. Headers: X-Passkey-Challenge e
// X-Passkey-Assertion (JSON da resposta do navegador, em base64url). Responde o erro se não conferir.
func (h *ConsentHandler) verifyPasskey(c *gin.Context, patientID, action string) bool {
	raw, err := base64.RawURLEncoding.DecodeString(c.GetHeader("X-Passkey-Assertion"))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "assinatura da passkey ausente"})
		return false
	}
	req, err := http.NewRequest(http.MethodPost, "/", bytes.NewReader(raw))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "falha ao ler assinatura"})
		return false
	}
	if err := h.passkeys.VerifyAction(c.Request.Context(), patientID, c.GetHeader("X-Passkey-Challenge"), action, req); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return false
	}
	return true
}

// resolvePatient usa o claim sub (userID injetado pelo AuthMiddleware) para achar o paciente.
func (h *ConsentHandler) resolvePatient(c *gin.Context) (string, bool) {
	patientID, err := h.sync.PatientIDByKeycloakID(c.Request.Context(), c.GetString("userID"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "paciente não encontrado para este usuário"})
		return "", false
	}
	return patientID, true
}

type grantConsentRequest struct {
	Scope      string     `json:"scope" binding:"required"`
	ValidUntil *time.Time `json:"valid_until,omitempty"`
}

// GrantConsent POST /api/consents
func (h *ConsentHandler) GrantConsent(c *gin.Context) {
	patientID, ok := h.resolvePatient(c)
	if !ok {
		return
	}
	var req grantConsentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "payload inválido: " + err.Error()})
		return
	}
	if !h.verifyPasskey(c, patientID, "consent:grant:"+req.Scope) {
		return
	}
	consent, err := h.sync.GrantConsent(c.Request.Context(), patientID, req.Scope, req.ValidUntil)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, consent)
}

// ListConsents GET /api/consents
func (h *ConsentHandler) ListConsents(c *gin.Context) {
	patientID, ok := h.resolvePatient(c)
	if !ok {
		return
	}
	list, err := h.sync.ListConsents(c.Request.Context(), patientID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "falha ao listar consentimentos"})
		return
	}
	c.JSON(http.StatusOK, list)
}

// RevokeConsent DELETE /api/consents/:id
func (h *ConsentHandler) RevokeConsent(c *gin.Context) {
	patientID, ok := h.resolvePatient(c)
	if !ok {
		return
	}
	if !h.verifyPasskey(c, patientID, "consent:revoke:"+c.Param("id")) {
		return
	}
	if err := h.sync.RevokeConsent(c.Request.Context(), patientID, c.Param("id")); err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, services.ErrNoConsent) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "revoked"})
}

// ListMyAudit GET /api/audit — quem acessou os dados do paciente autenticado (transparência LGPD).
func (h *ConsentHandler) ListMyAudit(c *gin.Context) {
	patientID, ok := h.resolvePatient(c)
	if !ok {
		return
	}
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if err != nil || limit < 1 || limit > 200 {
		limit = 50
	}
	entries, err := h.audit.ListByPatient(patientID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "falha ao consultar auditoria"})
		return
	}
	c.JSON(http.StatusOK, entries)
}

package http

import (
	"errors"
	"net/http"

	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/services/users/core/services"
	"github.com/gin-gonic/gin"
)

// AccessCodeHandler: o paciente gera um código no app (com passkey) e a clínica o usa para liberar o histórico por um prazo curto.
type AccessCodeHandler struct {
	codes    *services.AccessCodeService
	sync     *services.ClinicalSyncService
	passkeys *services.PasskeyService
	audit    *services.AccessAuditService
}

func NewAccessCodeHandler(codes *services.AccessCodeService, sync *services.ClinicalSyncService, passkeys *services.PasskeyService, audit *services.AccessAuditService) *AccessCodeHandler {
	return &AccessCodeHandler{codes: codes, sync: sync, passkeys: passkeys, audit: audit}
}

// Generate POST /api/access-codes (paciente logado; exige assinatura da passkey para a ação "access-code:generate").
func (h *AccessCodeHandler) Generate(c *gin.Context) {
	patientID, err := h.sync.PatientIDByKeycloakID(c.Request.Context(), c.GetString("userID"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "paciente não encontrado para este usuário"})
		return
	}
	if !verifyPasskeyAssertion(c, h.passkeys, patientID, "access-code:generate") {
		return
	}
	code, expires, err := h.codes.Generate(c.Request.Context(), patientID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "falha ao gerar código"})
		return
	}
	if err := h.audit.Record("paciente:"+patientID, "access_code_generate", patientID, "ok"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "não foi possível registrar a auditoria"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"code": code, "expires_at": expires})
}

type redeemAccessCodeRequest struct {
	CPF  string `json:"cpf" binding:"required,min=11,max=14"`
	Code string `json:"code" binding:"required,len=6,numeric"`
}

// Redeem POST /api/access-codes/redeem (clínica autenticada): troca o código do paciente por acesso temporário.
func (h *AccessCodeHandler) Redeem(c *gin.Context) {
	var req redeemAccessCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cpf e código de 6 dígitos são obrigatórios"})
		return
	}
	actor := c.GetString("clinicActor")
	patientID, validUntil, err := h.codes.Redeem(c.Request.Context(), req.CPF, req.Code)
	if err != nil {
		_ = h.audit.Record(actor, "access_code_redeem", patientID, "negado")
		status := http.StatusForbidden
		if !errors.Is(err, services.ErrAccessCodeInvalid) {
			status = http.StatusInternalServerError
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	if err := h.audit.Record(actor, "access_code_redeem", patientID, "ok"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "não foi possível registrar a auditoria"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"access_until": validUntil})
}

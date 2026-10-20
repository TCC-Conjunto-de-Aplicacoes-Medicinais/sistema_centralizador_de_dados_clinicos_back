package http

import (
	"errors"
	"net/http"

	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/services/users/core/services"
	"github.com/gin-gonic/gin"
)

// PasskeyHandler cadastra passkeys (WebAuthn) do paciente e cuida da recuperação por código de e-mail.
type PasskeyHandler struct {
	passkeys *services.PasskeyService
	sync     *services.ClinicalSyncService
}

func NewPasskeyHandler(passkeys *services.PasskeyService, sync *services.ClinicalSyncService) *PasskeyHandler {
	return &PasskeyHandler{passkeys: passkeys, sync: sync}
}

func (h *PasskeyHandler) resolvePatient(c *gin.Context) (string, bool) {
	patientID, err := h.sync.PatientIDByKeycloakID(c.Request.Context(), c.GetString("userID"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "paciente não encontrado para este usuário"})
		return "", false
	}
	return patientID, true
}

// BeginRegister POST /api/passkeys/register/begin (paciente logado).
func (h *PasskeyHandler) BeginRegister(c *gin.Context) {
	patientID, ok := h.resolvePatient(c)
	if !ok {
		return
	}
	h.sendRegistrationOptions(c, patientID)
}

// FinishRegister POST /api/passkeys/register/finish (header X-Passkey-Challenge + resposta do navegador no corpo).
func (h *PasskeyHandler) FinishRegister(c *gin.Context) {
	patientID, ok := h.resolvePatient(c)
	if !ok {
		return
	}
	if err := h.passkeys.FinishRegistration(c.Request.Context(), patientID, c.GetHeader("X-Passkey-Challenge"), c.Request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"status": "registered"})
}

type passkeyActionRequest struct {
	Action string `json:"action" binding:"required,max=200"`
}

// BeginAction POST /api/passkeys/action/begin: desafio para assinar uma ação (ex.: conceder consentimento).
func (h *PasskeyHandler) BeginAction(c *gin.Context) {
	patientID, ok := h.resolvePatient(c)
	if !ok {
		return
	}
	var req passkeyActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "payload inválido: " + err.Error()})
		return
	}
	assertion, challengeID, err := h.passkeys.BeginAction(c.Request.Context(), patientID, req.Action)
	if errors.Is(err, services.ErrNoPasskey) {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "falha ao iniciar assinatura"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"challenge_id": challengeID, "options": assertion})
}

type recoveryRequest struct {
	Email string `json:"email" binding:"required,email,max=150"`
}

type recoveryVerifyRequest struct {
	Email string `json:"email" binding:"required,email,max=150"`
	Code  string `json:"code" binding:"required,len=6,numeric"`
}

// RequestRecovery POST /api/passkeys/recovery/request (público): envia o código por e-mail.
func (h *PasskeyHandler) RequestRecovery(c *gin.Context) {
	var req recoveryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "e-mail inválido"})
		return
	}
	if err := h.passkeys.RequestRecoveryCode(c.Request.Context(), req.Email); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "falha ao enviar código"})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"status": "se o e-mail existir, um código foi enviado"})
}

// VerifyRecovery POST /api/passkeys/recovery/verify (público): troca o código por um token de cadastro.
func (h *PasskeyHandler) VerifyRecovery(c *gin.Context) {
	var req recoveryVerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "e-mail ou código inválido"})
		return
	}
	token, err := h.passkeys.VerifyRecoveryCode(c.Request.Context(), req.Email, req.Code)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"recovery_token": token})
}

// BeginRecoveryRegister POST /api/passkeys/recovery/register/begin (header X-Recovery-Token).
func (h *PasskeyHandler) BeginRecoveryRegister(c *gin.Context) {
	creation, challengeID, err := h.passkeys.BeginRecoveryRegistration(c.Request.Context(), c.GetHeader("X-Recovery-Token"))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"challenge_id": challengeID, "options": creation})
}

// FinishRecoveryRegister POST /api/passkeys/recovery/register/finish (headers X-Recovery-Token e X-Passkey-Challenge).
func (h *PasskeyHandler) FinishRecoveryRegister(c *gin.Context) {
	err := h.passkeys.FinishRecoveryRegistration(c.Request.Context(), c.GetHeader("X-Recovery-Token"), c.GetHeader("X-Passkey-Challenge"), c.Request)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"status": "registered"})
}

func (h *PasskeyHandler) sendRegistrationOptions(c *gin.Context, patientID string) {
	creation, challengeID, err := h.passkeys.BeginRegistration(c.Request.Context(), patientID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "falha ao iniciar cadastro da passkey"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"challenge_id": challengeID, "options": creation})
}

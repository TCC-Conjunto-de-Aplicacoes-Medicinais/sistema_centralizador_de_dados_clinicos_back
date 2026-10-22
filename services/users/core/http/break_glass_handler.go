package http

import (
	"errors"
	"net/http"

	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/services/users/core/services"
	"github.com/gin-gonic/gin"
)

type breakGlassRequest struct {
	Justification string `json:"justificativa" binding:"required,min=10,max=500"`
}

// BreakGlass POST /api/patients/:id/break-glass (clínica autenticada).
// Acesso sem consentimento, só para paciente de demonstração (@example.com), com justificativa obrigatória.
// A auditoria é gravada antes de o bundle sair; se ela falhar, nada é devolvido.
func (h *ClinicHandler) BreakGlass(c *gin.Context) {
	var req breakGlassRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "justificativa obrigatória (10 a 500 caracteres)"})
		return
	}
	patientID := c.Param("id")
	actor := c.GetString("clinicActor")

	bundle, err := h.sync.BuildDemoBundle(c.Request.Context(), patientID)
	if errors.Is(err, services.ErrNotDemoPatient) {
		_ = h.audit.Record(actor, "break_glass", patientID, "negado_nao_demo")
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}
	if errors.Is(err, services.ErrPatientNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "paciente não encontrado"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "falha ao montar bundle FHIR"})
		return
	}

	if err := h.audit.Record(actor, "break_glass", patientID, "ok: "+req.Justification); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "não foi possível registrar a auditoria; nada foi liberado"})
		return
	}
	c.JSON(http.StatusOK, bundle)
}

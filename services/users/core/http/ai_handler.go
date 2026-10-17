package http

import (
	"net/http"

	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/services/users/core/services"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/logger"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/models"
	"github.com/gin-gonic/gin"
)

type AIHandler struct {
	AIAnalysisService *services.AIAnalysisService
	Logger            *logger.Logger
}

func NewAIHandler(aiAnalysisService *services.AIAnalysisService, l *logger.Logger) *AIHandler {
	return &AIHandler{
		AIAnalysisService: aiAnalysisService,
		Logger:            l,
	}
}

// AIAnalyze
// @Summary      Análise de Exames por IA
// @Description  Envia um texto descrevendo exames ou sintomas e recebe uma análise gerada por IA (segunda opinião)
// @Tags         ai
// @Accept       json
// @Produce      json
// @Param        Authorization header   string                      true  "Access Token (Bearer)"
// @Param        DPoP          header   string                      true  "DPoP Proof JWT (RFC 9449)"
// @Param        request       body     models.AIAnalysisRequest    true  "Texto da consulta do paciente"
// @Success      200           {object} models.AIAnalysisResponse
// @Failure      400           {object} map[string]string
// @Failure      401           {object} map[string]string
// @Failure      500           {object} map[string]string
// @Router       /api/ai/analyze [post]
func (h *AIHandler) AIAnalyze(c *gin.Context) {
	if h.AIAnalysisService == nil {
		h.Logger.Log(logger.LogEntry{
			OriginService: "users",
			ActionType:    "ai_analyze",
			Description:   "tentativa de uso do serviço de IA, mas o serviço não está configurado",
			OriginIP:      c.ClientIP(),
			ResultStatus:  "warning",
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "serviço de IA indisponível"})
		return
	}

	id := c.GetString("userID")
	if id == "" {
		h.Logger.Log(logger.LogEntry{
			OriginService: "users",
			ActionType:    "ai_analyze",
			Description:   "tentativa de análise de IA sem usuário autenticado",
			OriginIP:      c.ClientIP(),
			ResultStatus:  "warning",
		})
		c.JSON(http.StatusUnauthorized, gin.H{"error": "usuário não identificado"})
		return
	}

	var req models.AIAnalysisRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.Logger.Log(logger.LogEntry{
			OriginService: "users",
			ActionType:    "ai_analyze",
			Description:   "payload inválido para análise de IA do usuário " + id + ": " + err.Error(),
			OriginIP:      c.ClientIP(),
			ResultStatus:  "error",
			UserID:        id,
		})
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Log: requisição recebida
	h.Logger.Log(logger.LogEntry{
		OriginService: "users",
		ActionType:    "ai_analyze",
		Description:   "requisição de análise de IA recebida do usuário " + id,
		OriginIP:      c.ClientIP(),
		ResultStatus:  "success",
		UserID:        id,
	})

	resp, err := h.AIAnalysisService.Analyze(c.Request.Context(), id, req)
	if err != nil {
		h.Logger.Log(logger.LogEntry{
			OriginService: "users",
			ActionType:    "ai_analyze",
			Description:   "erro na análise de IA para usuário " + id + ": " + err.Error(),
			OriginIP:      c.ClientIP(),
			ResultStatus:  "error",
			UserID:        id,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro ao processar análise: " + err.Error()})
		return
	}

	h.Logger.Log(logger.LogEntry{
		OriginService: "users",
		ActionType:    "ai_analyze",
		Description:   "análise de IA entregue com sucesso para usuário " + id,
		OriginIP:      c.ClientIP(),
		ResultStatus:  "success",
		UserID:        id,
	})
	c.JSON(http.StatusOK, resp)
}

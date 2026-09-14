package http

import (
	"net/http"

	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/database"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/logger"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ConsentHandler struct {
	DB     *gorm.DB
	Logger *logger.Logger
}

func NewConsentHandler(db *gorm.DB, l *logger.Logger) *ConsentHandler {
	return &ConsentHandler{DB: db, Logger: l}
}

// GET /api/users/consents - Lista permissões ativas de exames do paciente
func (h *ConsentHandler) ListConsents(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Usuário não autenticado"})
		return
	}

	var permissions []database.DoctorPermission
	err := h.DB.Preload("Doctor").Preload("Exam").
		Joins("JOIN exam ON exam.id = doctor_permission.exam_id").
		Where("exam.patient_id = ?", userID).
		Find(&permissions).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao listar consentimentos: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, permissions)
}

type ApproveConsentPayload struct {
	DoctorID      string `json:"doctorId" binding:"required"`
	ExamID        string `json:"examId" binding:"required"`
	DPoPSignature string `json:"dpopSignature"`
}

// POST /api/users/consents/approve - Paciente autoriza médico a visualizar exame
func (h *ConsentHandler) ApproveConsent(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Usuário não autenticado"})
		return
	}

	var req ApproveConsentPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Payload inválido: " + err.Error()})
		return
	}

	// Valida se o exame pertence ao paciente
	var exam database.Exam
	if err := h.DB.Where("id = ? AND patient_id = ?", req.ExamID, userID).First(&exam).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Exame não localizado ou paciente sem custódia"})
		return
	}

	permission := database.DoctorPermission{
		Id:            uuid.New().String(),
		DoctorID:      req.DoctorID,
		ExamID:        req.ExamID,
		BreakTheGlass: "AuthorizedByPatient_DPoP",
	}

	if err := h.DB.Create(&permission).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao gravar consentimento: " + err.Error()})
		return
	}

	if h.Logger != nil {
		h.Logger.Log(logger.LogEntry{
			OriginService: "users",
			ActionType:    "patient_grant_consent",
			Description:   "Paciente " + userID + " autorizou médico " + req.DoctorID + " para o exame " + req.ExamID,
			OriginIP:      c.ClientIP(),
			ResultStatus:  "success",
			UserID:        userID,
		})
	}

	c.JSON(http.StatusOK, gin.H{"message": "Consentimento concedido com sucesso", "permissionId": permission.Id})
}

// DELETE /api/users/consents/:id - Paciente revoga imediatamente o acesso
func (h *ConsentHandler) RevokeConsent(c *gin.Context) {
	userID := c.GetString("userID")
	permissionID := c.Param("id")

	var perm database.DoctorPermission
	err := h.DB.Joins("JOIN exam ON exam.id = doctor_permission.exam_id").
		Where("doctor_permission.id = ? AND exam.patient_id = ?", permissionID, userID).
		First(&perm).Error

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Permissão não encontrada ou sem autorização"})
		return
	}

	if err := h.DB.Delete(&perm).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao revogar permissão: " + err.Error()})
		return
	}

	if h.Logger != nil {
		h.Logger.Log(logger.LogEntry{
			OriginService: "users",
			ActionType:    "patient_revoke_consent",
			Description:   "Paciente " + userID + " revogou a permissão " + permissionID,
			OriginIP:      c.ClientIP(),
			ResultStatus:  "success",
			UserID:        userID,
		})
	}

	c.JSON(http.StatusOK, gin.H{"message": "Permissão revogada com sucesso"})
}

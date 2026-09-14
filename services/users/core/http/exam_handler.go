package http

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/database"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/logger"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ExamHandler struct {
	DB        *gorm.DB
	Logger    *logger.Logger
	UploadDir string
}

func NewExamHandler(db *gorm.DB, l *logger.Logger) *ExamHandler {
	uploadDir := os.Getenv("UPLOAD_DIR")
	if uploadDir == "" {
		uploadDir = "./uploads"
	}
	_ = os.MkdirAll(uploadDir, 0755)

	return &ExamHandler{
		DB:        db,
		Logger:    l,
		UploadDir: uploadDir,
	}
}

// GET /api/exams - Lista todos os exames ativos do paciente autenticado
func (h *ExamHandler) ListPatientExams(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Usuário não autenticado"})
		return
	}

	var exams []database.Exam
	err := h.DB.Where("patient_id = ? AND flag_active = ?", userID, true).
		Order("created_at DESC").
		Find(&exams).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao buscar exames: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, exams)
}

// GET /api/exams/:id - Detalhes de um exame específico
func (h *ExamHandler) GetExamByID(c *gin.Context) {
	userID := c.GetString("userID")
	examID := c.Param("id")

	var exam database.Exam
	err := h.DB.Where("id = ? AND patient_id = ? AND flag_active = ?", examID, userID, true).First(&exam).Error
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Exame não encontrado"})
		return
	}

	c.JSON(http.StatusOK, exam)
}

// POST /api/exams - Upload multipart de arquivo de exame
func (h *ExamHandler) UploadExam(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Usuário não autenticado"})
		return
	}

	title := c.PostForm("title")
	provider := c.PostForm("provider")
	results := c.PostForm("results")

	if title == "" {
		title = "Exame Clínico"
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Arquivo é obrigatório: " + err.Error()})
		return
	}
	defer file.Close()

	examID := uuid.New().String()
	safeFilename := fmt.Sprintf("%s_%s", examID, header.Filename)
	filePath := filepath.Join(h.UploadDir, safeFilename)

	out, err := os.Create(filePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Falha ao salvar arquivo no servidor"})
		return
	}
	defer out.Close()
	_, _ = io.Copy(out, file)

	exam := database.Exam{
		Id:          examID,
		PatientId:   userID,
		LinkBucket:  fmt.Sprintf("/api/exams/file/%s/%s", examID, header.Filename),
		IdCassandra: uuid.New().String(),
		FlagActive:  true,
		Title:       title,
		Provider:    provider,
		Result:      results,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := h.DB.Create(&exam).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao registrar exame no banco: " + err.Error()})
		return
	}

	if h.Logger != nil {
		h.Logger.Log(logger.LogEntry{
			OriginService: "users",
			ActionType:    "upload_exam",
			Description:   fmt.Sprintf("Paciente %s enviou novo exame %s (%s)", userID, examID, title),
			OriginIP:      c.ClientIP(),
			ResultStatus:  "success",
			UserID:        userID,
		})
	}

	c.JSON(http.StatusCreated, exam)
}

// GET /api/exams/file/:id/:filename - Download/Stream seguro do arquivo
func (h *ExamHandler) DownloadExamFile(c *gin.Context) {
	examID := c.Param("id")
	filename := c.Param("filename")

	safeFilename := fmt.Sprintf("%s_%s", examID, filename)
	filePath := filepath.Join(h.UploadDir, safeFilename)

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Arquivo físico não encontrado"})
		return
	}

	c.File(filePath)
}

// DELETE /api/exams/:id - Exclusão lógica (soft-delete)
func (h *ExamHandler) DeleteExam(c *gin.Context) {
	userID := c.GetString("userID")
	examID := c.Param("id")

	result := h.DB.Model(&database.Exam{}).
		Where("id = ? AND patient_id = ?", examID, userID).
		Update("flag_active", false)

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao excluir exame: " + result.Error.Error()})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Exame não encontrado ou sem permissão"})
		return
	}

	if h.Logger != nil {
		h.Logger.Log(logger.LogEntry{
			OriginService: "users",
			ActionType:    "delete_exam",
			Description:   fmt.Sprintf("Paciente %s excluiu o exame %s", userID, examID),
			OriginIP:      c.ClientIP(),
			ResultStatus:  "success",
			UserID:        userID,
		})
	}

	c.JSON(http.StatusOK, gin.H{"message": "Exame excluído com sucesso"})
}

package http

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/services/users/core/services"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/logger"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/models"
	"github.com/gin-gonic/gin"
)

type ExamHandler struct {
	ExamService *services.ExamService
	Logger      *logger.Logger
}

func NewExamHandler(examService *services.ExamService, l *logger.Logger) *ExamHandler {
	return &ExamHandler{
		ExamService: examService,
		Logger:      l,
	}
}

// ShareExam
// @Summary      Compartilhar Exame
// @Description  Mock de compartilhamento de exame que gera um log no Cassandra
// @Tags         exams
// @Accept       json
// @Produce      json
// @Param        Authorization header   string                   true  "Access Token (Bearer)"
// @Param        DPoP          header   string                   true  "DPoP Proof JWT (RFC 9449)"
// @Param        request       body     models.ShareExamRequest  true  "Dados para compartilhamento"
// @Success      200           {object} map[string]string
// @Failure      400           {object} map[string]string
// @Failure      401           {object} map[string]string
// @Failure      500           {object} map[string]string
// @Router       /api/exams/share [post]
func (h *ExamHandler) ShareExam(c *gin.Context) {
	id := c.GetString("userID")
	if id == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "usuário não identificado"})
		return
	}

	var req models.ShareExamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.Logger.Log(logger.LogEntry{
			OriginService: "exams",
			ActionType:    "share_exam",
			Description:   "payload inválido para compartilhamento: " + err.Error(),
			OriginIP:      c.ClientIP(),
			ResultStatus:  "error",
		})
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Gera o log simulando o compartilhamento no Cassandra
	err := h.Logger.Log(logger.LogEntry{
		OriginService: "exams",
		ActionType:    "share_exam",
		Description:   "exame " + req.ExamID + " compartilhado com o médico " + req.DoctorName,
		OriginIP:      c.ClientIP(),
		ResultStatus:  "success",
		UserID:        id,
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro ao registrar log: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Exame compartilhado com sucesso!"})
}

// UploadExam
// @Summary      Upload de Exame
// @Description  Faz o upload de um arquivo de exame e salva os metadados no banco
// @Tags         exams
// @Accept       multipart/form-data
// @Produce      json
// @Param        file         formData file   true  "Arquivo do exame"
// @Param        date         formData string true  "Data do exame (YYYY-MM-DD ou formato RFC3339)"
// @Param        exam_type    formData string true  "Tipo de exame"
// @Param        institution  formData string false "Instituição"
// @Param        exam_result  formData string false "Resultado do exame"
// @Success      201          {object} map[string]interface{}
// @Failure      400          {object} map[string]string
// @Failure      401          {object} map[string]string
// @Failure      500          {object} map[string]string
// @Router       /api/exams [post]
func (h *ExamHandler) UploadExam(c *gin.Context) {
	patientID := c.GetString("userID")
	if patientID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "usuário não identificado"})
		return
	}

	// 1. Obtém o arquivo da requisição multipart
	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "campo 'file' ausente ou inválido: " + err.Error()})
		return
	}

	// 2. Obtém os outros parâmetros
	examType := c.PostForm("exam_type")
	if examType == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "campo 'exam_type' obrigatório"})
		return
	}

	dateStr := c.PostForm("date")
	if dateStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "campo 'date' obrigatório"})
		return
	}

	// Tenta parsear a data
	var examDate time.Time
	examDate, err = time.Parse("2006-01-02", dateStr)
	if err != nil {
		// Se falhar, tenta parsear como RFC3339
		examDate, err = time.Parse(time.RFC3339, dateStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "formato de data inválido. Use YYYY-MM-DD ou RFC3339"})
			return
		}
	}

	var institution *string
	if inst := c.PostForm("institution"); inst != "" {
		institution = &inst
	}

	var examResult *string
	if res := c.PostForm("exam_result"); res != "" {
		examResult = &res
	}

	// 3. Abre o arquivo
	file, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro ao abrir arquivo: " + err.Error()})
		return
	}
	defer file.Close()

	// 4. Executa o upload via serviço
	exam, err := h.ExamService.UploadExam(
		c.Request.Context(),
		patientID,
		file,
		fileHeader.Filename,
		fileHeader.Size,
		fileHeader.Header.Get("Content-Type"),
		examDate,
		examType,
		institution,
		examResult,
	)
	if err != nil {
		h.Logger.Log(logger.LogEntry{
			OriginService: "exams",
			ActionType:    "upload_exam",
			Description:   "erro ao fazer upload de exame para paciente " + patientID + ": " + err.Error(),
			OriginIP:      c.ClientIP(),
			ResultStatus:  "error",
			UserID:        patientID,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.Logger.Log(logger.LogEntry{
		OriginService: "exams",
		ActionType:    "upload_exam",
		Description:   "exame " + exam.Id + " carregado com sucesso para paciente " + patientID,
		OriginIP:      c.ClientIP(),
		ResultStatus:  "success",
		UserID:        patientID,
	})

	c.JSON(http.StatusCreated, gin.H{
		"message": "Exame adicionado com sucesso!",
		"exam":    exam,
	})
}

// GetExamFile
// @Summary      Download de Arquivo de Exame
// @Summary      Obter Stream do Arquivo de Exame com DPoP
// @Description  Obtém o stream do arquivo de exame armazenado no MinIO exigindo DPoP proof no header
// @Tags         exams
// @Param        id            path     string true "ID do Exame"
// @Param        filename      path     string true "Nome do Arquivo"
// @Param        Authorization header   string true "Access Token (Bearer)"
// @Param        DPoP          header   string true "DPoP Proof JWT (RFC 9449)"
// @Success      200           {file}   binary
// @Failure      401           {object} map[string]string
// @Failure      403           {object} map[string]string
// @Router       /api/exams/file/{id}/{filename} [get]
func (h *ExamHandler) GetExamFile(c *gin.Context) {
	patientID := c.GetString("userID")
	if patientID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "usuário não identificado"})
		return
	}

	examID := c.Param("id")
	filename := c.Param("filename")

	if examID == "" || filename == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "parâmetros 'id' e 'filename' são obrigatórios"})
		return
	}

	fileStream, contentType, size, err := h.ExamService.GetExamFile(c.Request.Context(), patientID, examID, filename)
	if err != nil {
		h.Logger.Log(logger.LogEntry{
			OriginService: "exams",
			ActionType:    "download_exam",
			Description:   "erro de permissão ou arquivo não encontrado ao tentar baixar exame " + examID + ": " + err.Error(),
			OriginIP:      c.ClientIP(),
			ResultStatus:  "error",
			UserID:        patientID,
		})
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}
	defer fileStream.Close()

	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Header("Content-Type", contentType)
	c.Header("Content-Length", fmt.Sprintf("%d", size))

	_, _ = io.Copy(c.Writer, fileStream)
}

// GetExams
// @Summary      Listar Exames do Paciente
// @Description  Retorna todos os exames ativos do paciente logado
// @Tags         exams
// @Produce      json
// @Param        Authorization header   string  true  "Access Token (Bearer)"
// @Param        DPoP          header   string  true  "DPoP Proof JWT (RFC 9449)"
// @Success      200           {array}  database.Exam
// @Failure      401           {object} map[string]string
// @Failure      500           {object} map[string]string
// @Router       /api/exams [get]
func (h *ExamHandler) GetExams(c *gin.Context) {
	patientID := c.GetString("userID")
	if patientID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "usuário não identificado"})
		return
	}

	exams, err := h.ExamService.GetExams(c.Request.Context(), patientID)
	if err != nil {
		h.Logger.Log(logger.LogEntry{
			OriginService: "exams",
			ActionType:    "get_exams",
			Description:   "erro ao buscar exames para paciente " + patientID + ": " + err.Error(),
			OriginIP:      c.ClientIP(),
			ResultStatus:  "error",
			UserID:        patientID,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, exams)
}

// GetExamByID
// @Summary      Obter Detalhes do Exame
// @Description  Retorna as informações de um exame específico do paciente logado
// @Tags         exams
// @Produce      json
// @Param        id            path     string true  "ID do Exame"
// @Param        Authorization header   string  true  "Access Token (Bearer)"
// @Param        DPoP          header   string  true  "DPoP Proof JWT (RFC 9449)"
// @Success      200           {object} database.Exam
// @Failure      401           {object} map[string]string
// @Failure      403           {object} map[string]string
// @Failure      404           {object} map[string]string
// @Failure      500           {object} map[string]string
// @Router       /api/exams/{id} [get]
func (h *ExamHandler) GetExamByID(c *gin.Context) {
	patientID := c.GetString("userID")
	if patientID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "usuário não identificado"})
		return
	}

	examID := c.Param("id")
	if examID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "parâmetro 'id' é obrigatório"})
		return
	}

	exam, err := h.ExamService.GetExamByID(c.Request.Context(), patientID, examID)
	if err != nil {
		h.Logger.Log(logger.LogEntry{
			OriginService: "exams",
			ActionType:    "get_exam_by_id",
			Description:   "erro ao buscar exame " + examID + " para paciente " + patientID + ": " + err.Error(),
			OriginIP:      c.ClientIP(),
			ResultStatus:  "error",
			UserID:        patientID,
		})
		if strings.Contains(err.Error(), "acesso negado") {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, exam)
}

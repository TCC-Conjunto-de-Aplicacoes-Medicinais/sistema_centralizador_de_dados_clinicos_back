package http

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/services/users/core/services"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/database"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/logger"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/models"
	"github.com/gin-gonic/gin"
	"github.com/gocql/gocql"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ExamHandler struct {
	ExamService *services.ExamService
	DB          *gorm.DB
	Cassandra   *gocql.Session
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
	if err := h.Logger.Log(logger.LogEntry{
		OriginService: "exams",
		ActionType:    "share_exam",
		Description:   "exame " + req.ExamID + " compartilhado com o médico " + req.DoctorName,
		OriginIP:      c.ClientIP(),
		ResultStatus:  "success",
		UserID:        id,
	}); err != nil {
		fmt.Printf("⚠️ Erro ao registrar log no Cassandra (share_exam): %v\n", err)
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

// DeleteExam
// @Summary      Deletar Exame
// @Description  Exclui (soft delete) um exame pertencente ao paciente autenticado
// @Tags         exams
// @Param        id            path     string true  "ID do Exame"
// @Param        Authorization header   string  true  "Access Token (Bearer)"
// @Param        DPoP          header   string  true  "DPoP Proof JWT (RFC 9449)"
// @Success      200           {object} map[string]string
// @Failure      400           {object} map[string]string
// @Failure      401           {object} map[string]string
// @Failure      403           {object} map[string]string
// @Failure      404           {object} map[string]string
// @Failure      500           {object} map[string]string
// @Router       /api/exams/{id} [delete]
func (h *ExamHandler) DeleteExam(c *gin.Context) {
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

	err := h.ExamService.DeleteExam(c.Request.Context(), patientID, examID)
	if err != nil {
		h.Logger.Log(logger.LogEntry{
			OriginService: "exams",
			ActionType:    "delete_exam",
			Description:   "erro ao deletar exame " + examID + " para paciente " + patientID + ": " + err.Error(),
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

	h.Logger.Log(logger.LogEntry{
		OriginService: "exams",
		ActionType:    "delete_exam",
		Description:   "exame " + examID + " deletado com sucesso para paciente " + patientID,
		OriginIP:      c.ClientIP(),
		ResultStatus:  "success",
		UserID:        patientID,
	})

	c.JSON(http.StatusOK, gin.H{"message": "Exame deletado com sucesso!"})
}

// FHIRDiagnosticReport representa a estrutura padronizada HL7 FHIR Release 4 enviada pelas clínicas
type FHIRDiagnosticReport struct {
	ResourceType string `json:"resourceType"`
	ID           string `json:"id"`
	Status       string `json:"status"`
	Code         struct {
		Text   string `json:"text"`
		Coding []struct {
			System  string `json:"system"`
			Code    string `json:"code"`
			Display string `json:"display"`
		} `json:"coding"`
	} `json:"code"`
	Subject struct {
		Reference string `json:"reference"`
	} `json:"subject"`
	Performer []struct {
		Display   string `json:"display"`
		Reference string `json:"reference"`
	} `json:"performer"`
	EffectiveDateTime string `json:"effectiveDateTime"`
	Conclusion        string `json:"conclusion"`
	Notes             string `json:"notes"`
	Meta              struct {
		ClinicID string `json:"clinic_id"`
		Standard string `json:"standard"`
	} `json:"meta"`
}

// IngestExamFHIR permite que instâncias clínicas federadas despachem laudos e exames HL7 FHIR
func (h *ExamHandler) IngestExamFHIR(c *gin.Context) {
	var report FHIRDiagnosticReport
	if err := c.ShouldBindJSON(&report); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Payload JSON inválido: " + err.Error()})
		return
	}

	patientID := strings.TrimPrefix(report.Subject.Reference, "Patient/")
	if patientID == "" {
		patientID = "1"
	}

	title := report.Code.Text
	if title == "" {
		title = "Laudo Clínico Interoperável"
	}

	provider := "Clínica POHINC"
	if len(report.Performer) > 0 && report.Performer[0].Display != "" {
		provider = report.Performer[0].Display
	}

	examID := report.ID
	if examID == "" {
		examID = uuid.New().String()
	}

	clinicID := report.Meta.ClinicID
	if clinicID == "" {
		clinicID = "CLINIC-FEDERATED"
	}

	exam := database.Exam{
		Id:          examID,
		PatientId:   patientID,
		ClinicId:    &clinicID,
		Date:        time.Now(),
		ExamType:    "HL7-FHIR-DiagnosticReport",
		LinkBucket:  fmt.Sprintf("/api/exams/fhir/%s", examID),
		IdCassandra: uuid.New().String(),
		FlagActive:  true,
		Title:       title,
		Provider:    provider,
		Result:      report.Conclusion,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if h.DB != nil {
		if err := h.DB.Create(&exam).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao persistir exame no banco central: " + err.Error()})
			return
		}
	}

	// Persistência em Apache Cassandra (Pluralidade e Séries Temporais Descentralizadas)
	if h.Cassandra != nil {
		payloadBytes, _ := json.Marshal(report)
		insertCQL := `INSERT INTO patient_clinical_records (
			patient_id, exam_timestamp, record_id, clinic_id, clinic_name, resource_type, title, conclusion, fhir_payload
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);`

		if err := h.Cassandra.Query(insertCQL,
			patientID,
			time.Now(),
			examID,
			clinicID,
			provider,
			"DiagnosticReport",
			title,
			report.Conclusion,
			string(payloadBytes),
		).Exec(); err != nil {
			log.Printf("⚠️ Aviso: falha ao persistir série clínica no Cassandra: %v", err)
		} else {
			log.Printf(" Cassandra: Laudo FHIR %s da clínica '%s' persistido com sucesso para o paciente %s", examID, provider, patientID)
		}
	}

	if h.Logger != nil {
		h.Logger.Log(logger.LogEntry{
			OriginService: "users",
			ActionType:    "ingest_fhir_exam",
			Description:   fmt.Sprintf("Clínica %s despachou exame FHIR %s (%s) para paciente %s", clinicID, examID, title, patientID),
			OriginIP:      c.ClientIP(),
			ResultStatus:  "success",
			UserID:        patientID,
		})
	}

	c.JSON(http.StatusCreated, gin.H{
		"status":    "ingested",
		"id":        exam.Id,
		"standard":  "HL7-FHIR-R4",
		"plurality": "Cassandra + MariaDB",
	})
}

// GET /api/patients/:id/timeline - Retorna a linha do tempo cumulativa do Cassandra (Pluralidade de Múltiplas Clínicas)
func (h *ExamHandler) GetPatientTimeline(c *gin.Context) {
	patientID := c.Param("id")
	if h.Cassandra == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Cassandra não disponível"})
		return
	}

	iter := h.Cassandra.Query(
		`SELECT patient_id, exam_timestamp, record_id, clinic_id, clinic_name, resource_type, title, conclusion, fhir_payload 
		 FROM patient_clinical_records WHERE patient_id = ?`,
		patientID,
	).Iter()

	var timeline []gin.H
	var pID, recID, cID, cName, resType, title, conclusion, payload string
	var tStamp time.Time

	for iter.Scan(&pID, &tStamp, &recID, &cID, &cName, &resType, &title, &conclusion, &payload) {
		timeline = append(timeline, gin.H{
			"patient_id":     pID,
			"exam_timestamp": tStamp.Format(time.RFC3339),
			"record_id":      recID,
			"clinic_id":      cID,
			"clinic_name":    cName,
			"resource_type":  resType,
			"title":          title,
			"conclusion":     conclusion,
			"fhir_payload":   payload,
		})
	}
	if err := iter.Close(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao consultar timeline no Cassandra: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"patient_id": patientID,
		"plurality":  "Cassandra Multi-Clinic Timeline",
		"total":      len(timeline),
		"records":    timeline,
	})
}

// GET /api/patients/:id/fhir-bundle - Retorna o prontuário federado no formato HL7 FHIR Bundle (Release 4)
func (h *ExamHandler) GetPatientFHIRBundle(c *gin.Context) {
	patientID := c.Param("id")

	var entries []gin.H

	// 1. Informações básicas do Paciente
	entries = append(entries, gin.H{
		"fullUrl": fmt.Sprintf("urn:uuid:%s", patientID),
		"resource": gin.H{
			"resourceType": "Patient",
			"id":           patientID,
			"active":       true,
		},
	})

	// 2. Registros clínicos e laudos persistidos na arquitetura poliglota (Cassandra)
	if h.Cassandra != nil {
		iter := h.Cassandra.Query(
			`SELECT patient_id, exam_timestamp, record_id, clinic_id, clinic_name, resource_type, title, conclusion, fhir_payload 
			 FROM patient_clinical_records WHERE patient_id = ?`,
			patientID,
		).Iter()

		var pID, recID, cID, cName, resType, title, conclusion, payload string
		var tStamp time.Time

		for iter.Scan(&pID, &tStamp, &recID, &cID, &cName, &resType, &title, &conclusion, &payload) {
			var parsedPayload map[string]interface{}
			if err := json.Unmarshal([]byte(payload), &parsedPayload); err == nil && len(parsedPayload) > 0 {
				entries = append(entries, gin.H{
					"fullUrl":  fmt.Sprintf("urn:uuid:%s", recID),
					"resource": parsedPayload,
				})
			} else {
				entries = append(entries, gin.H{
					"fullUrl": fmt.Sprintf("urn:uuid:%s", recID),
					"resource": gin.H{
						"resourceType": "DiagnosticReport",
						"id":           recID,
						"status":       "final",
						"code": gin.H{
							"text": title,
						},
						"subject": gin.H{
							"reference": "Patient/" + patientID,
						},
						"effectiveDateTime": tStamp.Format(time.RFC3339),
						"conclusion":        conclusion,
						"performer": []gin.H{
							{"display": cName},
						},
					},
				})
			}
		}
		_ = iter.Close()
	}

	// 3. Empacota no padrão oficial HL7 FHIR Bundle (R4 Transaction)
	bundle := gin.H{
		"resourceType": "Bundle",
		"id":           fmt.Sprintf("bundle-%s-%d", patientID, time.Now().Unix()),
		"type":         "transaction",
		"timestamp":    time.Now().Format(time.RFC3339),
		"total":        len(entries),
		"entry":        entries,
	}

	c.JSON(http.StatusOK, bundle)
}

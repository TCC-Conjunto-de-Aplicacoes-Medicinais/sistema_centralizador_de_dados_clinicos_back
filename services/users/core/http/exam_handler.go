package http

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/database"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/logger"
	"github.com/gin-gonic/gin"
	"github.com/gocql/gocql"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ExamHandler struct {
	DB        *gorm.DB
	Cassandra *gocql.Session
	Logger    *logger.Logger
	UploadDir string
}

func NewExamHandler(db *gorm.DB, cassandra *gocql.Session, l *logger.Logger) *ExamHandler {
	uploadDir := os.Getenv("UPLOAD_DIR")
	if uploadDir == "" {
		uploadDir = "./uploads"
	}
	_ = os.MkdirAll(uploadDir, 0755)

	return &ExamHandler{
		DB:        db,
		Cassandra: cassandra,
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
		ClinicId:    clinicID,
		LinkBucket:  fmt.Sprintf("/api/exams/fhir/%s", examID),
		IdCassandra: uuid.New().String(),
		FlagActive:  true,
		Title:       title,
		Provider:    provider,
		Result:      report.Conclusion,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := h.DB.Create(&exam).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao persistir exame no banco central: " + err.Error()})
		return
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


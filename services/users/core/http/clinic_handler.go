package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/services/users/core/services"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/auth"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/database"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/logger"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type ClinicHandler struct {
	DB          *gorm.DB
	SMTPService *services.SmtpEmailService
	Logger      *logger.Logger
}

func NewClinicHandler(db *gorm.DB, smtpService *services.SmtpEmailService, l *logger.Logger) *ClinicHandler {
	return &ClinicHandler{
		DB:          db,
		SMTPService: smtpService,
		Logger:      l,
	}
}

type ClinicLoginRequest struct {
	ClinicCode string `json:"clinicCode" binding:"required"`
	Email      string `json:"email" binding:"required,email"`
	Password   string `json:"password" binding:"required"`
}

func (h *ClinicHandler) LoginClinic(c *gin.Context) {
	var req ClinicLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Payload inválido: " + err.Error()})
		return
	}

	var user database.ClinicalUser
	err := h.DB.Preload("Clinic").Where("email = ?", req.Email).First(&user).Error
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Credenciais inválidas. Usuário não encontrado."})
		return
	}

	if !user.Active {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Usuário inativo."})
		return
	}

	// Compara ClinicCode
	if strings.ToUpper(user.ClinicID) != strings.ToUpper(req.ClinicCode) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Credenciais inválidas. Código de clínica incorreto."})
		return
	}

	// Valida senha (suporta bcrypt ou texto claro para os mocks iniciais)
	pwdErr := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password))
	if pwdErr != nil && user.PasswordHash != req.Password {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Credenciais inválidas. Senha incorreta."})
		return
	}

	// Assina token JWT
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "centralizador_secret_chave_padrao"
	}

	claims := auth.ClinicClaims{
		ClinicID:       user.ClinicID,
		ClinicalUserID: user.ID,
		Email:          user.Email,
		FullName:       user.FullName,
		Role:           user.Role,
	}

	token, tokenErr := auth.GenerateClinicToken(claims, secret)
	if tokenErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao gerar token de acesso: " + tokenErr.Error()})
		return
	}

	h.Logger.Log(logger.LogEntry{
		OriginService: "users",
		ActionType:    "clinic_login",
		Description:   fmt.Sprintf("Profissional %s logou com sucesso na clínica %s", user.Email, user.ClinicID),
		OriginIP:      c.ClientIP(),
		ResultStatus:  "success",
	})

	c.JSON(http.StatusOK, gin.H{
		"token": token,
		"user": gin.H{
			"name": user.FullName,
			"role": user.Role,
		},
	})
}

func (h *ClinicHandler) SearchPatients(c *gin.Context) {
	cpf := c.Query("cpf")
	name := c.Query("name")

	if cpf == "" && name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Forneça o parâmetro 'cpf' ou 'name' para a busca."})
		return
	}

	var patients []database.Patient
	var queryErr error

	if cpf != "" {
		// Busca exata pelo CPF limpo (apenas dígitos)
		cleanCPF := strings.ReplaceAll(strings.ReplaceAll(cpf, ".", ""), "-", "")
		queryErr = h.DB.Where("cpf = ? OR REPLACE(REPLACE(cpf, '.', ''), '-', '') = ?", cpf, cleanCPF).Find(&patients).Error
	} else {
		// Busca parcial pelo Nome
		queryErr = h.DB.Where("name LIKE ?", "%"+name+"%").Find(&patients).Error
	}

	if queryErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao buscar paciente: " + queryErr.Error()})
		return
	}

	if len(patients) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"found": false,
		})
		return
	}

	// Retorna o primeiro encontrado para simplificar conforme o formato do front-end
	patient := patients[0]
	c.JSON(http.StatusOK, gin.H{
		"found": true,
		"patient": gin.H{
			"id":        patient.Id,
			"name":      patient.Name,
			"cpf":       patient.CPF,
			"birthDate": patient.BirthDate.Format("2006-01-02"),
		},
	})
}

type RequestDataPayload struct {
	PatientID        string `json:"patientId" binding:"required"`
	AuthMethod       string `json:"authMethod" binding:"required"` // "token" ou "break_the_glass"
	TokenCode        string `json:"tokenCode"`
	Justification    string `json:"justification"`
	RequesterDetails string `json:"requesterDetails"`
}

func (h *ClinicHandler) RequestPatientData(c *gin.Context) {
	clinicID := c.GetString("clinicID")
	requesterEmail := c.GetString("requesterEmail")

	var req RequestDataPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Payload inválido: " + err.Error()})
		return
	}

	// 1. Busca paciente
	var patient database.Patient
	err := h.DB.Preload("Emails").Preload("Phones").Preload("Exams").Where("id = ?", req.PatientID).First(&patient).Error
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Paciente não encontrado."})
		return
	}

	// 2. Valida autorização
	if req.AuthMethod == "token" {
		var token database.PatientToken
		tokenErr := h.DB.Where("patient_id = ? AND token_code = ? AND expires_at > ? AND used = false",
			patient.Id, req.TokenCode, time.Now()).First(&token).Error
		if tokenErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Código autorizador OTP inválido ou expirado."})
			return
		}

		// Consome o token
		token.Used = true
		h.DB.Save(&token)
	} else if req.AuthMethod == "break_the_glass" {
		if strings.TrimSpace(req.Justification) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "A justificativa médica é obrigatória para o protocolo 'Break the Glass'."})
			return
		}

		// Notifica o paciente em background
		patientEmail := patient.Id + "@gmail.com" // Fallback
		if len(patient.Emails) > 0 {
			for _, em := range patient.Emails {
				if em.Principal {
					patientEmail = em.Email
					break
				}
			}
		}

		var clinic database.Clinic
		var clinicName = clinicID
		if h.DB.Where("id = ?", clinicID).First(&clinic).Error == nil {
			clinicName = clinic.ClinicName
		}

		requesterDetails := req.RequesterDetails
		if requesterDetails == "" {
			requesterDetails = requesterEmail
		}

		go func() {
			ctx := context.Background()
			_ = h.SMTPService.SendEmergencyAccessAlert(ctx, patientEmail, clinicName, requesterDetails, req.Justification)
		}()
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Método de autorização desconhecido."})
		return
	}

	// 3. Grava log de auditoria
	var clinic database.Clinic
	var clinicName = clinicID
	if h.DB.Where("id = ?", clinicID).First(&clinic).Error == nil {
		clinicName = clinic.ClinicName
	}

	auditLog := database.AccessAuditLog{
		ClinicID:         clinicID,
		ClinicName:       clinicName,
		RequesterEmail:   requesterEmail,
		PatientID:        patient.Id,
		PatientName:      patient.Name,
		RequestType:      "direct", // Se for HL7 vira 'hl7_download'
		AuthMethod:       req.AuthMethod,
		Justification:    req.Justification,
		RequesterDetails: req.RequesterDetails,
	}

	if clinic.Type == "partner" {
		auditLog.RequestType = "hl7_download"
	}

	h.DB.Create(&auditLog)

	// 4. Estrutura o prontuário para resposta
	var allergies []string
	if patient.Allergies != "" {
		_ = json.Unmarshal([]byte(patient.Allergies), &allergies)
	}

	var medications []string
	if patient.Medications != "" {
		_ = json.Unmarshal([]byte(patient.Medications), &medications)
	}

	// Formata exames
	var examsList []gin.H
	for _, ex := range patient.Exams {
		if ex.FlagActive {
			examsList = append(examsList, gin.H{
				"id":       ex.Id,
				"title":    ex.Title,
				"date":     ex.CreatedAt.Format("2006-01-02"),
				"provider": ex.Provider,
				"result":   ex.Result,
			})
		}
	}

	// Retorna prontuário
	c.JSON(http.StatusOK, gin.H{
		"patient": gin.H{
			"id":          patient.Id,
			"name":        patient.Name,
			"cpf":         patient.CPF,
			"birthDate":   patient.BirthDate.Format("2006-01-02"),
			"allergies":   allergies,
			"medications": medications,
			"exams":       examsList,
		},
		"log": gin.H{
			"clinicName":     clinicName,
			"requesterEmail": requesterEmail,
			"patientName":    patient.Name,
			"authMethod":     req.AuthMethod,
			"requestType":    auditLog.RequestType,
			"justification":  req.Justification,
			"timestamp":      time.Now().Format(time.RFC3339),
		},
	})
}

func (h *ClinicHandler) ExportPatientHL7(c *gin.Context) {
	patientID := c.Param("id")
	clinicID := c.GetString("clinicID")
	requesterEmail := c.GetString("requesterEmail")

	var patient database.Patient
	err := h.DB.Preload("Emails").Preload("Phones").Preload("Exams").Where("id = ?", patientID).First(&patient).Error
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Paciente não encontrado."})
		return
	}

	// Cria log de auditoria
	var clinic database.Clinic
	var clinicName = clinicID
	if h.DB.Where("id = ?", clinicID).First(&clinic).Error == nil {
		clinicName = clinic.ClinicName
	}

	auditLog := database.AccessAuditLog{
		ClinicID:       clinicID,
		ClinicName:     clinicName,
		RequesterEmail: requesterEmail,
		PatientID:      patient.Id,
		PatientName:    patient.Name,
		RequestType:    "hl7_download",
		AuthMethod:     "token", // Considera token no fluxo de exportação direta do prontuário
	}
	h.DB.Create(&auditLog)

	timestamp := time.Now().Format(time.RFC3339)

	var phone = ""
	if len(patient.Phones) > 0 {
		phone = patient.Phones[0].Phone
	}
	var email = ""
	if len(patient.Emails) > 0 {
		email = patient.Emails[0].Email
	}

	// Monta entradas do FHIR
	entries := []gin.H{
		{
			"resource": gin.H{
				"resourceType": "Patient",
				"id":           patient.Id,
				"active":       true,
				"identifier": []gin.H{
					{
						"use":    "official",
						"system": "http://cadunico.gov.br/cpf",
						"value":  patient.CPF,
					},
				},
				"name": []gin.H{
					{
						"use":  "official",
						"text": patient.Name,
					},
				},
				"telecom": []gin.H{
					{
						"system": "phone",
						"value":  phone,
						"use":    "mobile",
					},
					{
						"system": "email",
						"value":  email,
					},
				},
				"birthDate": patient.BirthDate.Format("2006-01-02"),
			},
		},
	}

	// Alergias
	var allergies []string
	if patient.Allergies != "" {
		_ = json.Unmarshal([]byte(patient.Allergies), &allergies)
	}
	for i, all := range allergies {
		entries = append(entries, gin.H{
			"resource": gin.H{
				"resourceType": "AllergyIntolerance",
				"id":           fmt.Sprintf("%s-all-%d", patient.Id, i+1),
				"clinicalStatus": gin.H{
					"coding": []gin.H{
						{
							"system": "http://terminology.hl7.org/CodeSystem/allergyintolerance-clinical",
							"code":   "active",
						},
					},
				},
				"verificationStatus": gin.H{
					"coding": []gin.H{
						{
							"system": "http://terminology.hl7.org/CodeSystem/allergyintolerance-verification",
							"code":   "confirmed",
						},
					},
				},
				"category": []string{"medication", "environment"},
				"code": gin.H{
					"text": all,
				},
				"patient": gin.H{
					"reference": "Patient/" + patient.Id,
				},
			},
		})
	}

	// Medicamentos
	var medications []string
	if patient.Medications != "" {
		_ = json.Unmarshal([]byte(patient.Medications), &medications)
	}
	for i, med := range medications {
		entries = append(entries, gin.H{
			"resource": gin.H{
				"resourceType":              "MedicationStatement",
				"id":                        fmt.Sprintf("%s-med-%d", patient.Id, i+1),
				"status":                    "active",
				"medicationCodeableConcept": gin.H{"text": med},
				"subject":                   gin.H{"reference": "Patient/" + patient.Id},
				"dateAsserted":              timestamp,
			},
		})
	}

	// Exames
	for _, ex := range patient.Exams {
		if ex.FlagActive {
			entries = append(entries, gin.H{
				"resource": gin.H{
					"resourceType": "DiagnosticReport",
					"id":           ex.Id,
					"status":       "final",
					"code":         gin.H{"text": ex.Title},
					"subject":      gin.H{"reference": "Patient/" + patient.Id},
					"issued":       ex.CreatedAt.Format(time.RFC3339),
					"performer": []gin.H{
						{"display": ex.Provider},
					},
					"conclusion": ex.Result,
				},
			})
		}
	}

	bundle := gin.H{
		"resourceType": "Bundle",
		"id":           fmt.Sprintf("bundle-%s-%d", patient.Id, time.Now().Unix()),
		"type":         "document",
		"timestamp":    timestamp,
		"entry":        entries,
	}

	c.JSON(http.StatusOK, bundle)
}

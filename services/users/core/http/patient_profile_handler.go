package http

import (
	"net/http"

	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/services/users/core/services"
	"github.com/gin-gonic/gin"
)

// PatientProfileHandler: o próprio paciente lê e edita o cadastro (telefone, convênio, tipo sanguíneo, alergias).
type PatientProfileHandler struct {
	sync *services.ClinicalSyncService
}

func NewPatientProfileHandler(sync *services.ClinicalSyncService) *PatientProfileHandler {
	return &PatientProfileHandler{sync: sync}
}

// Get GET /api/patients/me
func (h *PatientProfileHandler) Get(c *gin.Context) {
	patientID, err := h.sync.PatientIDByKeycloakID(c.Request.Context(), c.GetString("userID"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "paciente não encontrado para este usuário"})
		return
	}
	p, err := h.sync.GetPatientProfile(c.Request.Context(), patientID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "paciente não encontrado"})
		return
	}
	phone := ""
	for _, ph := range p.Phones {
		if ph.Principal {
			phone = ph.Phone
		}
	}
	birth := ""
	if !p.BirthDate.IsZero() {
		birth = p.BirthDate.Format("2006-01-02")
	}
	c.JSON(http.StatusOK, gin.H{
		"birth_date": birth,
		"name":       p.Name,
		"cpf":        p.CPF,
		"phone":      phone,
		"insurance":  p.Insurance,
		"blood_type": p.BloodType,
		"allergies":  p.Allergies,
	})
}

type updateProfileRequest struct {
	BirthDate *string `json:"birth_date"`
	Phone     *string `json:"phone"`
	Insurance *string `json:"insurance"`
	BloodType *string `json:"blood_type"`
	Allergies *string `json:"allergies"`
}

// Update PUT /api/patients/me
func (h *PatientProfileHandler) Update(c *gin.Context) {
	patientID, err := h.sync.PatientIDByKeycloakID(c.Request.Context(), c.GetString("userID"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "paciente não encontrado para este usuário"})
		return
	}
	var req updateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "payload inválido"})
		return
	}
	if err := h.sync.UpdatePatientProfile(c.Request.Context(), patientID, services.PatientProfileInput{
		BirthDate: req.BirthDate,
		Phone:     req.Phone,
		Insurance: req.Insurance,
		BloodType: req.BloodType,
		Allergies: req.Allergies,
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "não foi possível salvar o cadastro"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// ClinicRecords GET /api/patients/me/clinic-records — laudos e exames que as clínicas enviaram sobre o próprio paciente.
func (h *PatientProfileHandler) ClinicRecords(c *gin.Context) {
	patientID, err := h.sync.PatientIDByKeycloakID(c.Request.Context(), c.GetString("userID"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "paciente não encontrado para este usuário"})
		return
	}
	list, err := h.sync.PatientClinicRecords(c.Request.Context(), patientID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "não foi possível carregar os laudos"})
		return
	}
	c.JSON(http.StatusOK, list)
}

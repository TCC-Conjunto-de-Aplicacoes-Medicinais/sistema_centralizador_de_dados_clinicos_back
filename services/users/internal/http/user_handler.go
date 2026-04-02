package http

import (
	"net/http"

	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/services/users/internal/services"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/models"
	"github.com/gin-gonic/gin"
)

type UserHandler struct {
	SignupService *services.SignupService
}

func NewUserHandler(signupService *services.SignupService) *UserHandler {
	return &UserHandler{SignupService: signupService}
}

// @Summary      Cadastro de Paciente
// @Description  Cadastra um paciente integrando Keycloak, MariaDB e Cassandra
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        request body models.SignupRequest true "Dados do paciente e dispositivo"
// @Success      201  {object} map[string]string
// @Failure      400  {object} map[string]string
// @Failure      500  {object} map[string]string
// @Router       /signup [post]
func (h *UserHandler) Signup(c *gin.Context) {
	var req models.SignupRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.SignupService.Signup(req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Paciente cadastrado com sucesso!"})
}

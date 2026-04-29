package http

import (
	"net/http"
	"strings"

	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/services/users/core/services"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/logger"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/models"
	"github.com/gin-gonic/gin"
)

type UserHandler struct {
	SignupService *services.SignupService
	LoginService  *services.LoginService
	Logger        *logger.Logger
}

func NewUserHandler(signupService *services.SignupService, loginService *services.LoginService, l *logger.Logger) *UserHandler {
	return &UserHandler{
		SignupService: signupService,
		LoginService:  loginService,
		Logger:        l,
	}
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
// @Router       /api/signup [post]
func (h *UserHandler) Signup(c *gin.Context) {
	var req models.SignupRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		h.Logger.Log(logger.LogEntry{
			OriginService: "users",
			ActionType:    "signup",
			Description:   "payload inválido: " + err.Error(),
			OriginIP:      c.ClientIP(),
			ResultStatus:  "error",
		})
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.SignupService.Signup(req); err != nil {
		h.Logger.Log(logger.LogEntry{
			OriginService: "users",
			ActionType:    "signup",
			Description:   "falha ao cadastrar paciente: " + err.Error(),
			OriginIP:      c.ClientIP(),
			ResultStatus:  "error",
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.Logger.Log(logger.LogEntry{
		OriginService: "users",
		ActionType:    "signup",
		Description:   "paciente cadastrado com sucesso: " + req.Email,
		OriginIP:      c.ClientIP(),
		ResultStatus:  "success",
	})
	c.JSON(http.StatusCreated, gin.H{"message": "Paciente cadastrado com sucesso!"})
}

// @Summary      Login de Paciente com DPoP
// @Description  Autentica um paciente via Keycloak exigindo DPoP proof (RFC 9449) no header
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        DPoP    header   string             true  "DPoP Proof JWT (RFC 9449)"
// @Param        request body     models.LoginRequest true  "Credenciais do paciente"
// @Success      200     {object} models.LoginResponse
// @Failure      400     {object} map[string]string
// @Failure      401     {object} map[string]string
// @Failure      500     {object} map[string]string
// @Router       /api/login [post]
func (h *UserHandler) Login(c *gin.Context) {
	proofJWT := c.GetHeader("DPoP")

	var req models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.Logger.Log(logger.LogEntry{
			OriginService: "users",
			ActionType:    "login",
			Description:   "payload inválido: " + err.Error(),
			OriginIP:      c.ClientIP(),
			ResultStatus:  "error",
		})
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.LoginService.Login(proofJWT, req)
	if err != nil {
		msg := err.Error()
		h.Logger.Log(logger.LogEntry{
			OriginService: "users",
			ActionType:    "login",
			Description:   "falha no login (" + req.Email + "): " + msg,
			OriginIP:      c.ClientIP(),
			ResultStatus:  "error",
		})
		if strings.Contains(msg, "dpop") || strings.Contains(msg, "DPoP") ||
			strings.Contains(msg, "credenciais") || strings.Contains(msg, "header DPoP") {
			c.JSON(http.StatusUnauthorized, gin.H{"error": msg})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": msg})
		return
	}

	h.Logger.Log(logger.LogEntry{
		OriginService: "users",
		ActionType:    "login",
		Description:   "login realizado com sucesso: " + req.Email,
		OriginIP:      c.ClientIP(),
		ResultStatus:  "success",
	})
	c.JSON(http.StatusOK, resp)
}

// @Summary      Atualização de Access Token
// @Description  Emite um novo token usando um RefreshToken via Keycloak exigindo DPoP proof no header
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        DPoP    header   string             true  "DPoP Proof JWT (RFC 9449)"
// @Param        request body     models.RefreshRequest true  "Token de refresh"
// @Success      200     {object} models.RefreshResponse
// @Failure      400     {object} map[string]string
// @Failure      401     {object} map[string]string
// @Failure      500     {object} map[string]string
// @Router       /api/refresh [post]
func (h *UserHandler) Refresh(c *gin.Context) {
	proofJWT := c.GetHeader("DPoP")

	var req models.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.Logger.Log(logger.LogEntry{
			OriginService: "users",
			ActionType:    "refresh",
			Description:   "payload inválido: " + err.Error(),
			OriginIP:      c.ClientIP(),
			ResultStatus:  "error",
		})
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.LoginService.Refresh(proofJWT, req)
	if err != nil {
		msg := err.Error()
		h.Logger.Log(logger.LogEntry{
			OriginService: "users",
			ActionType:    "refresh",
			Description:   "falha ao renovar token: " + msg,
			OriginIP:      c.ClientIP(),
			ResultStatus:  "error",
		})
		if strings.Contains(msg, "dpop") || strings.Contains(msg, "DPoP") ||
			strings.Contains(msg, "inválidas") || strings.Contains(msg, "header DPoP") {
			c.JSON(http.StatusUnauthorized, gin.H{"error": msg})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": msg})
		return
	}

	h.Logger.Log(logger.LogEntry{
		OriginService: "users",
		ActionType:    "refresh",
		Description:   "token renovado com sucesso",
		OriginIP:      c.ClientIP(),
		ResultStatus:  "success",
	})
	c.JSON(http.StatusOK, resp)
}

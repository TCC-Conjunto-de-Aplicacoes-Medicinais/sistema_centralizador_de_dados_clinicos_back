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
	SignupService      *services.SignupService
	LoginService       *services.LoginService
	UpdateUserService  *services.UpdateUserService
	VerifyEmailService *services.VerifyEmailService
	Logger             *logger.Logger
}

func NewUserHandler(
	signupService *services.SignupService,
	loginService *services.LoginService,
	updateUserService *services.UpdateUserService,
	verifyEmailService *services.VerifyEmailService,
	l *logger.Logger,
) *UserHandler {
	return &UserHandler{
		SignupService:      signupService,
		LoginService:       loginService,
		UpdateUserService:  updateUserService,
		VerifyEmailService: verifyEmailService,
		Logger:             l,
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
	if h.SignupService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "serviço indisponível"})
		return
	}
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
	if h.LoginService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "serviço indisponível"})
		return
	}
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
			Description:   "falha no login (" + req.CPF + "): " + msg,
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
		Description:   "login realizado com sucesso: " + req.CPF,
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
	if h.LoginService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "serviço indisponível"})
		return
	}
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

// @Summary      Atualização de Usuário
// @Description  Atualiza dados do usuário (nome, telefone, endereço) no MariaDB e Keycloak
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        DPoP    header   string                   true  "DPoP Proof JWT (RFC 9449)"
// @Param        id      path     string                   true  "ID do Usuário"
// @Param        request body     models.UpdateUserRequest true  "Dados para atualização"
// @Success      200     {object} map[string]string
// @Failure      400     {object} map[string]string
// @Failure      500     {object} map[string]string
// @Router       /api/users/{id} [put]
func (h *UserHandler) UpdateUser(c *gin.Context) {
	if h.UpdateUserService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "serviço indisponível"})
		return
	}
	proofJWT := c.GetHeader("DPoP")
	id := c.Param("id")

	var req models.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.Logger.Log(logger.LogEntry{
			OriginService: "users",
			ActionType:    "update_user",
			Description:   "payload inválido para usuário " + id + ": " + err.Error(),
			OriginIP:      c.ClientIP(),
			ResultStatus:  "error",
		})
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.UpdateUserService.UpdateUser(proofJWT, id, req); err != nil {
		msg := err.Error()
		h.Logger.Log(logger.LogEntry{
			OriginService: "users",
			ActionType:    "update_user",
			Description:   "erro ao atualizar usuário " + id + ": " + msg,
			OriginIP:      c.ClientIP(),
			ResultStatus:  "error",
		})
		
		if strings.Contains(msg, "dpop") || strings.Contains(msg, "DPoP") || strings.Contains(msg, "header DPoP") {
			c.JSON(http.StatusUnauthorized, gin.H{"error": msg})
			return
		}
		
		c.JSON(http.StatusInternalServerError, gin.H{"error": msg})
		return
	}

	h.Logger.Log(logger.LogEntry{
		OriginService: "users",
		ActionType:    "update_user",
		Description:   "usuário " + id + " atualizado com sucesso",
		OriginIP:      c.ClientIP(),
		ResultStatus:  "success",
	})
	c.JSON(http.StatusOK, gin.H{"message": "Dados atualizados com sucesso"})
}

// @Summary      Enviar E-mail de Verificação
// @Description  Solicita ao Keycloak o envio de um e-mail de verificação para o usuário
// @Tags         users
// @Produce      json
// @Param        id      path     string  true  "ID do Usuário"
// @Success      202     {object} map[string]string
// @Failure      400     {object} map[string]string
// @Failure      500     {object} map[string]string
// @Router       /api/users/{id}/send-verify-email [post]
func (h *UserHandler) SendVerifyEmail(c *gin.Context) {
	if h.VerifyEmailService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "serviço indisponível"})
		return
	}
	id := c.Param("id")

	if err := h.VerifyEmailService.SendVerificationEmail(id); err != nil {
		h.Logger.Log(logger.LogEntry{
			OriginService: "users",
			ActionType:    "send_verify_email",
			Description:   "erro ao disparar e-mail para usuário " + id + ": " + err.Error(),
			OriginIP:      c.ClientIP(),
			ResultStatus:  "error",
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.Logger.Log(logger.LogEntry{
		OriginService: "users",
		ActionType:    "send_verify_email",
		Description:   "e-mail de verificação disparado para usuário " + id,
		OriginIP:      c.ClientIP(),
		ResultStatus:  "success",
	})
	c.JSON(http.StatusAccepted, gin.H{"message": "E-mail de verificação enviado com sucesso"})
}

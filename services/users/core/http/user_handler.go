package http

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/services/users/core/services"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/auth"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/logger"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/models"
	"github.com/gin-gonic/gin"
)

type UserHandler struct {
	SignupService      *services.SignupService
	LoginService       *services.LoginService
	UpdateUserService  *services.UpdateUserService
	VerifyEmailService *services.VerifyEmailService
	GetUserService     *services.GetUserService
	AIAnalysisService  *services.AIAnalysisService
	ExamService        *services.ExamService
	Logger             *logger.Logger
}

func NewUserHandler(
	signupService *services.SignupService,
	loginService *services.LoginService,
	updateUserService *services.UpdateUserService,
	verifyEmailService *services.VerifyEmailService,
	getUserService *services.GetUserService,
	aiAnalysisService *services.AIAnalysisService,
	examService *services.ExamService,
	l *logger.Logger,
) *UserHandler {
	return &UserHandler{
		SignupService:      signupService,
		LoginService:       loginService,
		UpdateUserService:  updateUserService,
		VerifyEmailService: verifyEmailService,
		GetUserService:     getUserService,
		AIAnalysisService:  aiAnalysisService,
		ExamService:        examService,
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
// @Tags         auth
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

	resp, err := h.LoginService.Login(req)
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

	// Extrai ID do Keycloak do token para logar o sucesso
	claims, _ := auth.ExtractUserClaims("Bearer " + resp.AccessToken)
	userID := ""
	if claims != nil {
		userID = claims.PatientID
	}

	h.Logger.Log(logger.LogEntry{
		OriginService: "users",
		ActionType:    "login",
		Description:   "login realizado com sucesso: " + req.CPF,
		OriginIP:      c.ClientIP(),
		ResultStatus:  "success",
		UserID:        userID,
	})
	c.JSON(http.StatusOK, resp)
}

// @Summary      Atualização de Access Token
// @Description  Emite um novo token usando um RefreshToken via Keycloak exigindo DPoP proof no header
// @Tags         auth
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

	resp, err := h.LoginService.Refresh(req)
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
// @Param        Authorization header   string                   true  "Access Token (Bearer)"
// @Param        DPoP          header   string                   true  "DPoP Proof JWT (RFC 9449)"
// @Param        request       body     models.UpdateUserRequest true  "Dados para atualização"
// @Success      200           {object} map[string]string
// @Failure      400           {object} map[string]string
// @Failure      401           {object} map[string]string
// @Failure      500           {object} map[string]string
// @Router       /api/users [put]
func (h *UserHandler) UpdateUser(c *gin.Context) {
	if h.UpdateUserService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "serviço indisponível"})
		return
	}
	// O ID e a validação DPoP já foram processados pelos middlewares
	id := c.GetString("userID")

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

	if err := h.UpdateUserService.UpdateUser(id, req); err != nil {
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
		UserID:        id,
	})
	c.JSON(http.StatusOK, gin.H{"message": "Dados atualizados com sucesso"})
}

// @Summary      Perfil do Usuário
// @Description  Retorna os dados editáveis do perfil do paciente (nome, telefone, endereço)
// @Tags         users
// @Produce      json
// @Param        Authorization header   string  true  "Access Token (Bearer)"
// @Param        DPoP          header   string  true  "DPoP Proof JWT (RFC 9449)"
// @Success      200     {object} services.UserProfileResponse
// @Failure      401     {object} map[string]string
// @Failure      500     {object} map[string]string
// @Router       /api/users/profile [get]
func (h *UserHandler) GetUserProfile(c *gin.Context) {
	if h.GetUserService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "serviço indisponível"})
		return
	}
	id := c.GetString("userID")
	if id == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "usuário não identificado"})
		return
	}

	profile, err := h.GetUserService.GetProfile(id)
	if err != nil {
		h.Logger.Log(logger.LogEntry{
			OriginService: "users",
			ActionType:    "get_profile",
			Description:   "erro ao buscar perfil do usuário " + id + ": " + err.Error(),
			OriginIP:      c.ClientIP(),
			ResultStatus:  "error",
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.Logger.Log(logger.LogEntry{
		OriginService: "users",
		ActionType:    "get_profile",
		Description:   "perfil do usuário " + id + " consultado com sucesso",
		OriginIP:      c.ClientIP(),
		ResultStatus:  "success",
		UserID:        id,
	})
	c.JSON(http.StatusOK, profile)
}

// @Summary      Enviar E-mail de Verificação
// @Description  Solicita ao Keycloak o envio de um e-mail de verificação para o usuário
// @Tags         auth
// @Produce      json
// @Param        Authorization header   string  true  "Access Token (Bearer)"
// @Param        DPoP          header   string  true  "DPoP Proof JWT (RFC 9449)"
// @Success      202     {object} map[string]string
// @Failure      400     {object} map[string]string
// @Failure      500     {object} map[string]string
// @Router       /api/users/send-verify-email [post]
func (h *UserHandler) SendVerifyEmail(c *gin.Context) {
	if h.VerifyEmailService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "serviço indisponível"})
		return
	}
	id := c.GetString("userID")
	if id == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "usuário não identificado"})
		return
	}

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
		UserID:        id,
	})
	c.JSON(http.StatusAccepted, gin.H{"message": "E-mail de verificação enviado com sucesso"})
}

type VerifyCodeRequest struct {
	Code string `json:"code" binding:"required"`
}

// @Summary      Validar E-mail
// @Description  Valida o código de verificação enviado por e-mail
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        Authorization header   string  true  "Access Token (Bearer)"
// @Param        DPoP          header   string  true  "DPoP Proof JWT (RFC 9449)"
// @Param        request body     VerifyCodeRequest true "Código de verificação"
// @Success      200     {object} map[string]string
// @Failure      400     {object} map[string]string
// @Failure      401     {object} map[string]string
// @Failure      500     {object} map[string]string
// @Router       /api/users/verify-email-code [post]
func (h *UserHandler) VerifyCode(c *gin.Context) {
	if h.VerifyEmailService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "serviço indisponível"})
		return
	}
	id := c.GetString("userID")
	if id == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "usuário não identificado"})
		return
	}

	var req VerifyCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.Logger.Log(logger.LogEntry{
			OriginService: "users",
			ActionType:    "verify_email",
			Description:   "payload inválido para verificação de e-mail: " + err.Error(),
			OriginIP:      c.ClientIP(),
			ResultStatus:  "error",
		})
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.VerifyEmailService.VerifyCode(id, req.Code); err != nil {
		h.Logger.Log(logger.LogEntry{
			OriginService: "users",
			ActionType:    "verify_email",
			Description:   "erro ao verificar e-mail do usuário " + id + ": " + err.Error(),
			OriginIP:      c.ClientIP(),
			ResultStatus:  "error",
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.Logger.Log(logger.LogEntry{
		OriginService: "users",
		ActionType:    "verify_email",
		Description:   "e-mail verificado com sucesso para usuário " + id,
		OriginIP:      c.ClientIP(),
		ResultStatus:  "success",
		UserID:        id,
	})
	c.JSON(http.StatusOK, gin.H{"message": "E-mail verificado com sucesso"})
}

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
// @Router       /api/users/exams/share [post]
func (h *UserHandler) ShareExam(c *gin.Context) {
	id := c.GetString("userID")
	if id == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "usuário não identificado"})
		return
	}

	var req models.ShareExamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.Logger.Log(logger.LogEntry{
			OriginService: "users",
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
		OriginService: "users",
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

// @Summary      Análise de Exames por IA
// @Description  Envia um texto descrevendo exames ou sintomas e recebe uma análise gerada por IA (segunda opinião)
// @Tags         ai
// @Accept       json
// @Produce      json
// @Param        Authorization header   string                      true  "Access Token (Bearer)"
// @Param        DPoP          header   string                      true  "DPoP Proof JWT (RFC 9449)"
// @Param        request       body     models.AIAnalysisRequest    true  "Texto da consulta do paciente"
// @Success      200           {object} models.AIAnalysisResponse
// @Failure      400           {object} map[string]string
// @Failure      401           {object} map[string]string
// @Failure      500           {object} map[string]string
// @Router       /api/ai/analyze [post]
func (h *UserHandler) AIAnalyze(c *gin.Context) {
	if h.AIAnalysisService == nil {
		h.Logger.Log(logger.LogEntry{
			OriginService: "users",
			ActionType:    "ai_analyze",
			Description:   "tentativa de uso do serviço de IA, mas o serviço não está configurado",
			OriginIP:      c.ClientIP(),
			ResultStatus:  "warning",
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "serviço de IA indisponível"})
		return
	}

	id := c.GetString("userID")
	if id == "" {
		h.Logger.Log(logger.LogEntry{
			OriginService: "users",
			ActionType:    "ai_analyze",
			Description:   "tentativa de análise de IA sem usuário autenticado",
			OriginIP:      c.ClientIP(),
			ResultStatus:  "warning",
		})
		c.JSON(http.StatusUnauthorized, gin.H{"error": "usuário não identificado"})
		return
	}

	var req models.AIAnalysisRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.Logger.Log(logger.LogEntry{
			OriginService: "users",
			ActionType:    "ai_analyze",
			Description:   "payload inválido para análise de IA do usuário " + id + ": " + err.Error(),
			OriginIP:      c.ClientIP(),
			ResultStatus:  "error",
			UserID:        id,
		})
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Log: requisição recebida
	h.Logger.Log(logger.LogEntry{
		OriginService: "users",
		ActionType:    "ai_analyze",
		Description:   "requisição de análise de IA recebida do usuário " + id,
		OriginIP:      c.ClientIP(),
		ResultStatus:  "success",
		UserID:        id,
	})

	resp, err := h.AIAnalysisService.Analyze(id, req)
	if err != nil {
		h.Logger.Log(logger.LogEntry{
			OriginService: "users",
			ActionType:    "ai_analyze",
			Description:   "erro na análise de IA para usuário " + id + ": " + err.Error(),
			OriginIP:      c.ClientIP(),
			ResultStatus:  "error",
			UserID:        id,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro ao processar análise: " + err.Error()})
		return
	}

	h.Logger.Log(logger.LogEntry{
		OriginService: "users",
		ActionType:    "ai_analyze",
		Description:   "análise de IA entregue com sucesso para usuário " + id,
		OriginIP:      c.ClientIP(),
		ResultStatus:  "success",
		UserID:        id,
	})
	c.JSON(http.StatusOK, resp)
}

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
func (h *UserHandler) UploadExam(c *gin.Context) {
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
			OriginService: "users",
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
		OriginService: "users",
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

// @Summary      Download de Arquivo de Exame
// @Description  Obtém o stream do arquivo de exame armazenado no MinIO
// @Tags         exams
// @Param        id           path     string true "ID do Exame"
// @Param        filename     path     string true "Nome do Arquivo"
// @Param        token        query    string false "JWT Token para autorização (opcional se enviado via Header)"
// @Success      200          {file}   binary
// @Failure      401          {object} map[string]string
// @Failure      403          {object} map[string]string
// @Failure      444          {object} map[string]string
// @Router       /api/exams/file/{id}/{filename} [get]
func (h *UserHandler) GetExamFile(c *gin.Context) {
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
			OriginService: "users",
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

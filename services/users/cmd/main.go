package main

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	userHttp "github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/services/users/core/http"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/services/users/core/services"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/services/users/core/usecase"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/config"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/database"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/dpop"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/logger"

	_ "github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/services/users/cmd/docs"
	"github.com/gin-contrib/cors"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"github.com/Nerzal/gocloak/v13"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

// @title           Sistema Centralizador de Dados Clínicos API
// @version         1.0
// @description     API para gerenciamento de pacientes e dispositivos via Keycloak e polyglot persistence.
// @host            api.pohinc.com.br
// @BasePath        /
func main() {
	envLoaded := false
	for _, envPath := range []string{".env", "../.env", "../../.env", "../../../.env"} {
		if err := godotenv.Load(envPath); err == nil {
			envLoaded = true
			break
		}
	}
	if !envLoaded {
		log.Println("Aviso: Arquivo .env não encontrado. Dependendo apenas de variáveis injetadas pelo sistema.")
	}

	keycloakAuth := config.InitKeycloak(
		os.Getenv("KEYCLOAK_URL"),
		os.Getenv("KEYCLOAK_CLIENT_ID"),
		os.Getenv("KEYCLOAK_CLIENT_SECRET"),
		os.Getenv("KEYCLOAK_REALM"),
	)

	// Clínicas ficam em outra realm (contas criadas pelo wizard). Rotas das clínicas e o login da clínica usam esta.
	// Sem o teste de login da InitKeycloak: uma configuração errada da clínica não derruba o app do paciente.
	clinicKeycloak := &config.KeycloakAuth{
		Client:       gocloak.NewClient(os.Getenv("KEYCLOAK_URL")),
		ClientID:     os.Getenv("KEYCLOAK_CLINIC_CLIENT_ID"),
		ClientSecret: os.Getenv("KEYCLOAK_CLINIC_CLIENT_SECRET"),
		Realm:        os.Getenv("KEYCLOAK_CLINIC_REALM"),
	}

	cassandraDB := config.CassandraConnect()
	defer cassandraDB.Close()

	if cassandraDB.Core == nil {
		log.Fatal("❌ Erro crítico: Não foi possível estabelecer as sessões mínimas do Cassandra.")
	}

	mariaDB := config.MariaDBConnect()
	if mariaDB == nil {
		log.Fatal("❌ Erro crítico: Falha ao iniciar sessão no MariaDB.")
	}

	if err := database.RunMigrations(mariaDB); err != nil {
		log.Fatalf("Erro ao rodar migrações do sistema central: %v", err)
	}

	if err := database.RunCassandraMigrations(cassandraDB.Core); err != nil {
		log.Fatalf("Erro ao criar tabelas do Cassandra: %v", err)
	}

	signupService := services.NewSignupService(mariaDB, cassandraDB, keycloakAuth)

	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8002"
	}
	replayStore := dpop.NewReplayStore(2 * time.Minute)
	dpopUseCase := usecase.NewValidateDPoPUseCase(replayStore, baseURL)
	loginService := services.NewLoginService(keycloakAuth, dpopUseCase)
	updateUserService := services.NewUpdateUserService(mariaDB, keycloakAuth, dpopUseCase)
	smtpPort, _ := strconv.Atoi(os.Getenv("SMTP_PORT"))
	if smtpPort == 0 {
		smtpPort = 587
	}
	smtpConfig := config.InitSMTP(
		os.Getenv("SMTP_HOST"),
		smtpPort,
		os.Getenv("SMTP_USER"),
		os.Getenv("SMTP_PASSWORD"),
	)
	smtpService := services.NewSMTPEmailService(smtpConfig)
	verifyEmailService := services.NewVerifyEmailService(mariaDB, keycloakAuth, smtpService)

	appLogger := logger.NewLogger(cassandraDB.Core)
	getUserService := services.NewGetUserService(mariaDB)
	aiAnalysisService := services.NewAIAnalysisService(mariaDB, os.Getenv("GEMINI_API_KEY"), appLogger)
	minioClient := config.MinIOConnect()
	examService := services.NewExamService(mariaDB, minioClient, baseURL)
	userHandler := userHttp.NewUserHandler(signupService, loginService, updateUserService, verifyEmailService, getUserService, appLogger)
	examHandler := userHttp.NewExamHandler(examService, appLogger)
	aiHandler := userHttp.NewAIHandler(aiAnalysisService, appLogger)
	clinicSync := services.NewClinicalSyncService(mariaDB, services.NewClinicalRecordStore(cassandraDB.Core))
	accessAudit := services.NewAccessAuditService(cassandraDB.Core)
	rpID := os.Getenv("WEBAUTHN_RP_ID")
	if rpID == "" {
		rpID = "localhost"
	}
	// Origens separadas por vírgula: https://app.pohinc.com.br, https://<rpID> (iOS), android:apk-key-hash:<hash> (Android).
	webauthnOrigins := strings.Split(os.Getenv("WEBAUTHN_ORIGINS"), ",")
	if os.Getenv("WEBAUTHN_ORIGINS") == "" {
		webauthnOrigins = []string{"http://localhost:8081"}
	}
	passkeyService, err := services.NewPasskeyService(mariaDB, smtpService, rpID, webauthnOrigins)
	if err != nil {
		log.Fatalf("Erro ao configurar passkeys: %v", err)
	}
	passkeyHandler := userHttp.NewPasskeyHandler(passkeyService, clinicSync)
	profileHandler := userHttp.NewPatientProfileHandler(clinicSync)
	accessCodeHandler := userHttp.NewAccessCodeHandler(services.NewAccessCodeService(mariaDB, clinicSync), clinicSync, passkeyService, accessAudit)
	clinicHandler := userHttp.NewClinicHandler(clinicSync, clinicKeycloak, accessAudit)
	consentHandler := userHttp.NewConsentHandler(clinicSync, accessAudit, passkeyService)

	router := gin.Default()

	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "DPoP", "X-Passkey-Challenge", "X-Passkey-Assertion", "X-Recovery-Token"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: false,
	}))

	// --- Rotas Públicas ---
	router.GET("/api/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"Message": "OK"})
	})
	router.POST("/api/signup", userHandler.Signup)
	router.POST("/api/clinic/login", clinicHandler.Login)
	router.POST("/api/clinic/refresh", clinicHandler.Refresh)
	router.POST("/api/passkeys/recovery/request", passkeyHandler.RequestRecovery)
	router.POST("/api/passkeys/recovery/verify", passkeyHandler.VerifyRecovery)
	router.POST("/api/passkeys/recovery/register/begin", passkeyHandler.BeginRecoveryRegister)
	router.POST("/api/passkeys/recovery/register/finish", passkeyHandler.FinishRecoveryRegister)

	// --- Rotas com DPoP Obrigatório (Login/Refresh) ---
	dpopGroup := router.Group("/api")
	dpopGroup.Use(userHttp.DPoPMiddleware(dpopUseCase, appLogger))
	{
		dpopGroup.POST("/login", userHandler.Login)
		dpopGroup.POST("/refresh", userHandler.Refresh)
	}

	// --- Rotas com Autenticação + DPoP (Perfil/Dados Sensíveis) ---
	authGroup := router.Group("/api")
	authGroup.Use(userHttp.DPoPMiddleware(dpopUseCase, appLogger))
	authGroup.Use(userHttp.AuthMiddleware(mariaDB, appLogger, keycloakAuth.IntrospectToken))
	{
		authGroup.GET("/users/profile", userHandler.GetUserProfile)
		authGroup.PUT("/users", userHandler.UpdateUser)
		authGroup.POST("/users/send-verify-email", userHandler.SendVerifyEmail)
		authGroup.POST("/users/verify-email-code", userHandler.VerifyCode)
		authGroup.POST("/exams/share", examHandler.ShareExam)
		authGroup.POST("/exams", examHandler.UploadExam)
		authGroup.GET("/exams", examHandler.GetExams)
		authGroup.GET("/exams/:id", examHandler.GetExamByID)
		authGroup.DELETE("/exams/:id", examHandler.DeleteExam)
		authGroup.POST("/ai/analyze", aiHandler.AIAnalyze)
		authGroup.GET("/exams/file/:id/:filename", examHandler.GetExamFile)
		authGroup.POST("/consents", consentHandler.GrantConsent)
		authGroup.GET("/consents", consentHandler.ListConsents)
		authGroup.DELETE("/consents/:id", consentHandler.RevokeConsent)
		authGroup.GET("/audit", consentHandler.ListMyAudit)
		authGroup.POST("/passkeys/register/begin", passkeyHandler.BeginRegister)
		authGroup.POST("/passkeys/register/finish", passkeyHandler.FinishRegister)
		authGroup.POST("/passkeys/action/begin", passkeyHandler.BeginAction)
		authGroup.POST("/access-codes", accessCodeHandler.Generate)
		authGroup.GET("/patients/me", profileHandler.Get)
		authGroup.PUT("/patients/me", profileHandler.Update)
	}

	// --- Rotas das clínicas federadas (X-Clinic-Key; sem DPoP de paciente) ---
	clinicGroup := router.Group("/api")
	clinicGroup.Use(userHttp.ClinicAccessMiddleware(os.Getenv("CLINIC_SYNC_KEY"), clinicKeycloak.VerifyAccessToken))
	{
		clinicGroup.GET("/patients/search", clinicHandler.SearchPatients)
		clinicGroup.POST("/patients/sync", clinicHandler.SyncPatient)
		clinicGroup.POST("/exams/ingest", clinicHandler.IngestRecord)
		clinicGroup.POST("/access-codes/redeem", accessCodeHandler.Redeem)
		clinicGroup.POST("/patients/:id/break-glass", clinicHandler.BreakGlass)
		clinicGroup.GET("/patients/:id/fhir-bundle", clinicHandler.FHIRBundle)
		clinicGroup.GET("/patients/:id/audit", clinicHandler.PatientAudit)
	}

	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	if err := router.Run(":8002"); err != nil {
		log.Fatal("Error starting server: ", err)
	}
}

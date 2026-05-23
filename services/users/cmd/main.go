package main

import (
	"log"
	"net/http"
	"os"
	"strconv"
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
	userHandler := userHttp.NewUserHandler(signupService, loginService, updateUserService, verifyEmailService, getUserService, appLogger)

	router := gin.Default()

	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "DPoP"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: false,
	}))

	// --- Rotas Públicas ---
	router.GET("/api/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"Message": "OK"})
	})
	router.POST("/api/signup", userHandler.Signup)

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
	authGroup.Use(userHttp.AuthMiddleware(mariaDB, appLogger))
	{
		authGroup.GET("/users/profile", userHandler.GetUserProfile)
		authGroup.PUT("/users", userHandler.UpdateUser)
		authGroup.POST("/users/send-verify-email", userHandler.SendVerifyEmail)
		authGroup.POST("/users/verify-email-code", userHandler.VerifyCode)
	}

	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	if err := router.Run(":8002"); err != nil {
		log.Fatal("Error starting server: ", err)
	}
}

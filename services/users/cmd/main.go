package main

import (
	"log"
	"net/http"

	"os"

	userHttp "github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/services/users/internal/http"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/services/users/internal/services"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/config"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/models"

	_ "github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/services/users/cmd/docs"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

// @title           Sistema Centralizador de Dados Clínicos API
// @version         1.0
// @description     API para gerenciamento de pacientes e dispositivos via Keycloak e polyglot persistence.
// @host            localhost:8000
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

	if err := models.RunMigrations(mariaDB); err != nil {
		log.Fatalf("Erro ao rodar migrações do sistema central: %v", err)
	}

	signupService := services.NewSignupService(mariaDB, cassandraDB, keycloakAuth)
	userHandler := userHttp.NewUserHandler(signupService)

	router := gin.Default()

	router.GET("/api/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"Message": "OK",
		})
	})

	router.POST("/api/signup", userHandler.Signup)

	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	if err := router.Run(":8000"); err != nil {
		log.Fatal("Error starting server: ", err)
	}
}

package main

import (
	"log"
	"net/http"

	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/config"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/models"
)

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

	config.InitKeycloak(
		os.Getenv("KEYCLOAK_URL"),
		os.Getenv("KEYCLOAK_CLIENT_ID"),
		os.Getenv("KEYCLOAK_CLIENT_SECRET"),
		os.Getenv("KEYCLOAK_REALM"),
	)

	// ips := []string{
	// 	os.Getenv("CASSANDRA_IP_LOCAL"),
	// 	os.Getenv("CASSANDRA_IP_MASTER"),
	// }
	// localDC := os.Getenv("CASSANDRA_LOCAL_DC")
	// clinicaKeyspace := os.Getenv("CASSANDRA_CLINICA_KEYSPACE")

	// if localDC == "" || clinicaKeyspace == "" {
	// 	log.Fatal("❌ Erro crítico: Configure as variáveis CASSANDRA_LOCAL_DC e CASSANDRA_CLINICA_KEYSPACE no .env desta Clínica.")
	// }

	// db := config.CassandraConnect(ips, localDC, clinicaKeyspace)
	// defer db.Close()

	// if db.Core == nil || db.Clinica == nil {
    //     log.Fatal("❌ Erro crítico: Não foi possível estabelecer as sessões mínimas do Cassandra.")
    // }

	// Inicializa sua engine Relacional GORM
	mariaDB := config.MariaDBConnect()
	if mariaDB == nil {
		log.Fatal("❌ Erro crítico: Falha ao iniciar sessão no MariaDB.")
	}

	// Executa a suíte de Migrate de todas as tabelas centralizadas
	if err := models.RunMigrations(mariaDB); err != nil {
		log.Fatalf("Erro ao rodar migrações do sistema central: %v", err)
	}

	router := gin.Default()

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"Message": "OK",
		})
	})

	if err := router.Run(":8000"); err != nil {
		log.Fatal("Error starting server: ", err)
	}
}
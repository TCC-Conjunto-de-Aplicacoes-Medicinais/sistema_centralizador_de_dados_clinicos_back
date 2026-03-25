package main

import (
	"log"
	"net/http"

	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/config"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("Aviso: Arquivo .env não encontrado. Dependendo apenas de variáveis injetadas pelo sistema.")
	}

	ips := []string{
		os.Getenv("CASSANDRA_IP_LOCAL"),
		os.Getenv("CASSANDRA_IP_MASTER"),
	}
	localDC := os.Getenv("CASSANDRA_LOCAL_DC")
	clinicaKeyspace := os.Getenv("CASSANDRA_CLINICA_KEYSPACE")

	if localDC == "" || clinicaKeyspace == "" {
		log.Fatal("❌ Erro crítico: Configure as variáveis CASSANDRA_LOCAL_DC e CASSANDRA_CLINICA_KEYSPACE no .env desta Clínica.")
	}

	db := config.CassandraConnect(ips, localDC, clinicaKeyspace)
	defer db.Close()

	if db.Core == nil || db.Clinica == nil {
        log.Fatal("❌ Erro crítico: Não foi possível estabelecer as sessões mínimas.")
    }

	router := gin.Default()

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"Message": "OK",
		})
	})

	if err := router.Run(":8080"); err != nil {
		log.Fatal("Error starting server: ", err)
	}
}
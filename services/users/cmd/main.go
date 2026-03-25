package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/config"
)

var (
	// TODO: ADICIONAR IP DO CASSANDRA MASTER
	ips             = []string{"127.0.0.1"}
	localDC         = "DC_Clinica_A"
	clinicaKeyspace = "ks_clinica_a"
)

func main() {
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
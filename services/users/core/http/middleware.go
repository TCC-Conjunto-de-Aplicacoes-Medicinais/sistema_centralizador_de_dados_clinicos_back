package http

import (
	"net/http"
	"os"

	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/services/users/core/usecase"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/auth"
	"github.com/gin-gonic/gin"
)

// DPoPMiddleware valida o proof DPoP para qualquer rota.
func DPoPMiddleware(dpopUC *usecase.ValidateDPoPUseCase) gin.HandlerFunc {
	return func(c *gin.Context) {
		proofJWT := c.GetHeader("DPoP")
		if proofJWT == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "header DPoP ausente"})
			c.Abort()
			return
		}

		// Obtém a URL base (pode vir do .env ou ser reconstruída)
		baseURL := os.Getenv("BASE_URL")
		if baseURL == "" {
			baseURL = "https://api.pohinc.com.br"
		}
		
		// O HTU no DPoP deve ser o caminho completo chamado
		htu := baseURL + c.Request.URL.Path
		htm := c.Request.Method

		if err := dpopUC.Execute(proofJWT, htm, htu); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "falha na validação DPoP: " + err.Error()})
			c.Abort()
			return
		}

		c.Next()
	}
}

// AuthMiddleware extrai a identidade do usuário do JWT e injeta no contexto.
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		userID, err := auth.ExtractSubFromToken(authHeader)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "token inválido ou ausente: " + err.Error()})
			c.Abort()
			return
		}

		// Armazena o ID do Keycloak no contexto do Gin
		c.Set("userID", userID)
		c.Next()
	}
}

package http

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/services/users/core/usecase"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/auth"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/database"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/logger"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// DPoPMiddleware valida o proof DPoP para qualquer rota.
func DPoPMiddleware(dpopUC *usecase.ValidateDPoPUseCase, l *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		proofJWT := c.GetHeader("DPoP")
		if proofJWT == "" {
			l.Log(logger.LogEntry{
				OriginService: "users",
				ActionType:    "dpop_validation",
				Description:   "header DPoP ausente na rota: " + c.Request.URL.Path,
				OriginIP:      c.ClientIP(),
				ResultStatus:  "error",
			})
			c.JSON(http.StatusUnauthorized, gin.H{"error": "header DPoP ausente"})
			c.Abort()
			return
		}

		// Obtém a URL base injetada no UseCase e normaliza removendo eventuais barras finais
		baseURL := strings.TrimRight(dpopUC.BaseURL, "/")

		// O HTU no DPoP deve ser o caminho completo chamado (normalizado com barra inicial única)
		path := "/" + strings.TrimLeft(c.Request.URL.Path, "/")
		htu := baseURL + path
		htm := c.Request.Method

		if err := dpopUC.Execute(proofJWT, htm, htu); err != nil {
			l.Log(logger.LogEntry{
				OriginService: "users",
				ActionType:    "dpop_validation",
				Description:   "falha na validação DPoP: " + err.Error(),
				OriginIP:      c.ClientIP(),
				ResultStatus:  "error",
			})
			c.JSON(http.StatusUnauthorized, gin.H{"error": "falha na validação DPoP: " + err.Error()})
			c.Abort()
			return
		}

		c.Next()
	}
}

// TokenVerifier valida a assinatura e a vigência do access token (ex.: introspecção no Keycloak).
type TokenVerifier func(ctx context.Context, token string) error

// AuthMiddleware valida o access token com o verifier e injeta a identidade do usuário no contexto.
// Também busca o nome atualizado do usuário no MariaDB.
// Sem verifier configurado, todas as requisições são rejeitadas (fail closed).
func AuthMiddleware(db *gorm.DB, l *logger.Logger, verify TokenVerifier) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		claims, err := auth.ExtractUserClaims(authHeader)
		if err == nil {
			if verify == nil {
				err = errors.New("verificação de token não configurada")
			} else {
				rawToken := strings.SplitN(authHeader, " ", 2)[1]
				err = verify(c.Request.Context(), rawToken)
			}
		}
		if err != nil {
			l.Log(logger.LogEntry{
				OriginService: "users",
				ActionType:    "auth_identification",
				Description:   "falha ao identificar usuário via token: " + err.Error(),
				OriginIP:      c.ClientIP(),
				ResultStatus:  "error",
			})
			c.JSON(http.StatusUnauthorized, gin.H{"error": "token inválido ou ausente: " + err.Error()})
			c.Abort()
			return
		}

		// 1. Busca o nome atualizado no MariaDB usando o Keycloak ID (sub)
		if db != nil {
			var user database.Patient
			if err := db.Select("name").Where("keycloak_id = ?", claims.PatientID).First(&user).Error; err == nil {
				claims.Name = user.Name
				claims.FullName = user.Name // Define o nome completo vindo do banco
			}
		}

		// 2. Armazena as informações simplificadas no contexto do Gin
		c.Set("userID", claims.PatientID)
		c.Set("userName", claims.Name)
		c.Set("userFullName", claims.FullName)
		c.Set("userEmail", claims.Email)
		c.Set("emailVerified", claims.EmailVerified)
		c.Next()
	}
}

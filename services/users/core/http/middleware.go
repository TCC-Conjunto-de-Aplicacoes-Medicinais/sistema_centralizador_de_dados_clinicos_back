package http

import (
	"net/http"
	"os"

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

		// Obtém a URL base injetada no UseCase
		baseURL := dpopUC.BaseURL

		// O HTU no DPoP deve ser o caminho completo chamado
		htu := baseURL + c.Request.URL.Path
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

// AuthMiddleware extrai a identidade do usuário do JWT e injeta no contexto.
// Também busca o nome atualizado do usuário no MariaDB.
func AuthMiddleware(db *gorm.DB, l *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		claims, err := auth.ExtractUserClaims(authHeader)
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

// ClinicAuthMiddleware valida o token de clinicas/médicos e injeta as credenciais no contexto.
func ClinicAuthMiddleware(l *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			l.Log(logger.LogEntry{
				OriginService: "users",
				ActionType:    "clinic_auth",
				Description:   "header Authorization ausente",
				OriginIP:      c.ClientIP(),
				ResultStatus:  "error",
			})
			c.JSON(http.StatusUnauthorized, gin.H{"error": "header Authorization ausente"})
			c.Abort()
			return
		}

		secret := os.Getenv("JWT_SECRET")
		if secret == "" {
			secret = "centralizador_secret_chave_padrao"
		}

		claims, err := auth.ValidateClinicToken(authHeader, secret)
		if err != nil {
			l.Log(logger.LogEntry{
				OriginService: "users",
				ActionType:    "clinic_auth",
				Description:   "token de clínica inválido ou expirado: " + err.Error(),
				OriginIP:      c.ClientIP(),
				ResultStatus:  "error",
			})
			c.JSON(http.StatusUnauthorized, gin.H{"error": "token de clínica inválido ou expirado: " + err.Error()})
			c.Abort()
			return
		}

		// Armazena as informações da clínica e médico no contexto
		c.Set("clinicID", claims.ClinicID)
		c.Set("clinicalUserID", claims.ClinicalUserID)
		c.Set("requesterEmail", claims.Email)
		c.Set("requesterName", claims.FullName)
		c.Set("requesterRole", claims.Role)
		c.Next()
	}
}

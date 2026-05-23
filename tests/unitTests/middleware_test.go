package unitTests

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	userHttp "github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/services/users/core/http"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/services/users/core/usecase"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/dpop"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
)

func TestAuthMiddleware_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Mock Logger (nulo para teste)
	router.Use(userHttp.AuthMiddleware(nil, nil))
	router.GET("/test", func(c *gin.Context) {
		userID := c.GetString("userID")
		userName := c.GetString("userName")
		emailVerified := c.GetBool("emailVerified")
		c.JSON(http.StatusOK, gin.H{
			"id":       userID,
			"name":     userName,
			"verified": emailVerified,
		})
	})

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":            "user-789",
		"name":           "Middleware User",
		"email_verified": true,
	})
	tokenString, _ := token.SignedString([]byte("secret"))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "user-789")
	assert.Contains(t, w.Body.String(), "Middleware User")
	assert.Contains(t, w.Body.String(), "true")
}

func TestAuthMiddleware_Failure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(userHttp.AuthMiddleware(nil, nil))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestDPoPMiddleware_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Setup real UseCase with dummy store
	store := dpop.NewReplayStore(time.Minute)
	uc := usecase.NewValidateDPoPUseCase(store, "http://example.com")

	router.Use(userHttp.DPoPMiddleware(uc, nil))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// Generate valid proof
	proof, _ := generateValidES256DPoPForURL("GET", "http://example.com/test")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("DPoP", proof)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// Helper para gerar DPoP específico para os testes de middleware
func generateValidES256DPoPForURL(htm, htu string) (string, string) {
	privateKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	jwk := map[string]interface{}{
		"kty": "EC", "crv": "P-256",
		"x": base64.RawURLEncoding.EncodeToString(privateKey.X.Bytes()),
		"y": base64.RawURLEncoding.EncodeToString(privateKey.Y.Bytes()),
	}
	jb, _ := json.Marshal(jwk)
	hdr, _ := json.Marshal(map[string]interface{}{"typ": "dpop+jwt", "alg": "ES256", "jwk": json.RawMessage(jb)})
	jti := "test-jti"
	claims, _ := json.Marshal(map[string]interface{}{
		"jti": jti, "htm": htm, "htu": htu, "iat": time.Now().Unix(),
	})
	
	input := base64.RawURLEncoding.EncodeToString(hdr) + "." + base64.RawURLEncoding.EncodeToString(claims)
	hash := sha256.Sum256([]byte(input))
	r, s, _ := ecdsa.Sign(rand.Reader, privateKey, hash[:])
	sig := make([]byte, 64)
	copy(sig[32-len(r.Bytes()):32], r.Bytes())
	copy(sig[64-len(s.Bytes()):], s.Bytes())
	
	return input + "." + base64.RawURLEncoding.EncodeToString(sig), jti
}

func TestDPoPMiddleware_Failure_MissingHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	store := dpop.NewReplayStore(time.Minute)
	uc := usecase.NewValidateDPoPUseCase(store, "http://example.com")

	router.Use(userHttp.DPoPMiddleware(uc, nil))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestDPoPMiddleware_Failure_InvalidProof(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	store := dpop.NewReplayStore(time.Minute)
	uc := usecase.NewValidateDPoPUseCase(store, "http://example.com")

	router.Use(userHttp.DPoPMiddleware(uc, nil))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("DPoP", "invalid.proof.jwt")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}


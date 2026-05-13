package unitTests

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"
	"math/big"

	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/dpop"
	"github.com/stretchr/testify/assert"
	"crypto/rsa"
)

func generateValidES256DPoP() (string, string) {
	privateKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	
	jwk := map[string]interface{}{
		"kty": "EC",
		"crv": "P-256",
		"x":   base64.RawURLEncoding.EncodeToString(privateKey.X.Bytes()),
		"y":   base64.RawURLEncoding.EncodeToString(privateKey.Y.Bytes()),
	}
	jwkBytes, _ := json.Marshal(jwk)

	hdr := map[string]interface{}{"typ": "dpop+jwt", "alg": "ES256", "jwk": json.RawMessage(jwkBytes)}
	hdrBytes, _ := json.Marshal(hdr)
	hdr64 := base64.RawURLEncoding.EncodeToString(hdrBytes)

	jti := "random-jti"
	claims := map[string]interface{}{
		"jti": jti,
		"htm": "POST",
		"htu": "http://localhost:8000/api/login",
		"iat": time.Now().Unix(),
	}
	claimsBytes, _ := json.Marshal(claims)
	claims64 := base64.RawURLEncoding.EncodeToString(claimsBytes)

	signingInput := hdr64 + "." + claims64
	hash := sha256.Sum256([]byte(signingInput))
	
	r, s, _ := ecdsa.Sign(rand.Reader, privateKey, hash[:])
	
	rBytes := r.Bytes()
	sBytes := s.Bytes()
	sigBytes := make([]byte, 64)
	copy(sigBytes[32-len(rBytes):32], rBytes)
	copy(sigBytes[64-len(sBytes):], sBytes)
	
	sig64 := base64.RawURLEncoding.EncodeToString(sigBytes)

	return signingInput + "." + sig64, jti
}

func TestParseAndValidate_InvalidFormat(t *testing.T) {
	_, err := dpop.ParseAndValidate("invalid.jwt", "POST", "http://localhost")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "formato JWT inválido")
}

func TestParseAndValidate_InvalidTyp(t *testing.T) {
	hdr := map[string]interface{}{"typ": "jwt", "alg": "ES256", "jwk": json.RawMessage(`{}`)}
	b, _ := json.Marshal(hdr)
	jwt := base64.RawURLEncoding.EncodeToString(b) + ".payload.sig"

	_, err := dpop.ParseAndValidate(jwt, "POST", "http://localhost")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "typ inválido")
}

func TestParseAndValidate_ValidEC(t *testing.T) {
	jwt, expectedJti := generateValidES256DPoP()

	jti, err := dpop.ParseAndValidate(jwt, "POST", "http://localhost:8000/api/login")
	assert.NoError(t, err)
	assert.Equal(t, expectedJti, jti)
}

func TestParseAndValidate_InvalidHTM(t *testing.T) {
	jwt, _ := generateValidES256DPoP()

	_, err := dpop.ParseAndValidate(jwt, "GET", "http://localhost:8000/api/login") 
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "htm")
}

func TestParseAndValidate_InvalidHTU(t *testing.T) {
	jwt, _ := generateValidES256DPoP()

	_, err := dpop.ParseAndValidate(jwt, "POST", "http://wrong-url.com") 
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "htu")
}

func TestParseAndValidate_MissingJWK(t *testing.T) {
	hdr := map[string]interface{}{"typ": "dpop+jwt", "alg": "ES256"}
	b, _ := json.Marshal(hdr)
	jwt := base64.RawURLEncoding.EncodeToString(b) + ".payload.sig"

	_, err := dpop.ParseAndValidate(jwt, "POST", "http://localhost")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "jwk ausente")
}

func TestParseAndValidate_InvalidAlg(t *testing.T) {
	hdr := map[string]interface{}{"typ": "dpop+jwt", "alg": "HS256"}
	b, _ := json.Marshal(hdr)
	jwt := base64.RawURLEncoding.EncodeToString(b) + ".payload.sig"

	_, err := dpop.ParseAndValidate(jwt, "POST", "http://localhost")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "algoritmo")
}

func TestParseAndValidate_MissingJTI(t *testing.T) {
	jwt, _ := generateValidES256DPoP()
	
	parts := strings.Split(jwt, ".")
	claims := map[string]interface{}{"htm": "POST", "iat": time.Now().Unix()} // sem JTI
	cb, _ := json.Marshal(claims)
	parts[1] = base64.RawURLEncoding.EncodeToString(cb)
	
	badJwt := strings.Join(parts, ".")
	_, err := dpop.ParseAndValidate(badJwt, "POST", "http://localhost:8000/api/login")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "jti ausente")
}

func TestParseAndValidate_ExpiredIAT(t *testing.T) {
	privateKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	jwk := map[string]interface{}{"kty": "EC", "crv": "P-256", "x": base64.RawURLEncoding.EncodeToString(privateKey.X.Bytes()), "y": base64.RawURLEncoding.EncodeToString(privateKey.Y.Bytes())}
	jwkBytes, _ := json.Marshal(jwk)

	hdr := map[string]interface{}{"typ": "dpop+jwt", "alg": "ES256", "jwk": json.RawMessage(jwkBytes)}
	hdrBytes, _ := json.Marshal(hdr)
	hdr64 := base64.RawURLEncoding.EncodeToString(hdrBytes)

	claims := map[string]interface{}{"jti": "random-jti", "htm": "POST", "htu": "http://localhost:8000/api/login", "iat": time.Now().Add(-10 * time.Minute).Unix()}
	claimsBytes, _ := json.Marshal(claims)
	claims64 := base64.RawURLEncoding.EncodeToString(claimsBytes)

	signingInput := hdr64 + "." + claims64
	hash := sha256.Sum256([]byte(signingInput))
	r, s, _ := ecdsa.Sign(rand.Reader, privateKey, hash[:])
	
	rBytes, sBytes := r.Bytes(), s.Bytes()
	sigBytes := make([]byte, 64)
	copy(sigBytes[32-len(rBytes):32], rBytes)
	copy(sigBytes[64-len(sBytes):], sBytes)
	
	jwt := signingInput + "." + base64.RawURLEncoding.EncodeToString(sigBytes)

	_, err := dpop.ParseAndValidate(jwt, "POST", "http://localhost:8000/api/login")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "iat fora da janela")
}

func TestParseAndValidate_ValidRS256(t *testing.T) {
	privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	jwk := map[string]interface{}{
		"kty": "RSA",
		"n":   base64.RawURLEncoding.EncodeToString(privateKey.N.Bytes()),
		"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(privateKey.E)).Bytes()),
	}
	jwkBytes, _ := json.Marshal(jwk)

	hdr := map[string]interface{}{"typ": "dpop+jwt", "alg": "RS256", "jwk": json.RawMessage(jwkBytes)}
	hdrBytes, _ := json.Marshal(hdr)
	hdr64 := base64.RawURLEncoding.EncodeToString(hdrBytes)

	claims := map[string]interface{}{"jti": "rsa-jti", "htm": "POST", "htu": "http://localhost:8000/api/login", "iat": time.Now().Unix()}
	claimsBytes, _ := json.Marshal(claims)
	claims64 := base64.RawURLEncoding.EncodeToString(claimsBytes)

	signingInput := hdr64 + "." + claims64
	hash := sha256.Sum256([]byte(signingInput))
	sigBytes, _ := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, hash[:])
	jwt := signingInput + "." + base64.RawURLEncoding.EncodeToString(sigBytes)

	jti, err := dpop.ParseAndValidate(jwt, "POST", "http://localhost:8000/api/login")
	assert.NoError(t, err)
	assert.Equal(t, "rsa-jti", jti)
}

func TestParseAndValidate_RSAKeyTooShort(t *testing.T) {
	privateKey, _ := rsa.GenerateKey(rand.Reader, 1024) // < 2048
	jwk := map[string]interface{}{
		"kty": "RSA",
		"n":   base64.RawURLEncoding.EncodeToString(privateKey.N.Bytes()),
		"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(privateKey.E)).Bytes()),
	}
	jwkBytes, _ := json.Marshal(jwk)
	hdr := map[string]interface{}{"typ": "dpop+jwt", "alg": "RS256", "jwk": json.RawMessage(jwkBytes)}
	hb, _ := json.Marshal(hdr)
	jwt := base64.RawURLEncoding.EncodeToString(hb) + ".payload.sig"

	_, err := dpop.ParseAndValidate(jwt, "POST", "http://localhost")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "rejeitada")
}

func TestParseAndValidate_InvalidKTY(t *testing.T) {
	jwk := map[string]interface{}{"kty": "oct"}
	jb, _ := json.Marshal(jwk)
	hdr := map[string]interface{}{"typ": "dpop+jwt", "alg": "ES256", "jwk": json.RawMessage(jb)}
	hb, _ := json.Marshal(hdr)
	jwt := base64.RawURLEncoding.EncodeToString(hb) + ".payload.sig"

	_, err := dpop.ParseAndValidate(jwt, "POST", "http://localhost")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "kty \"oct\" não suportado")
}


package dpop

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	_ "crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/url"
	"strings"
	"time"
)

type rawHeader struct {
	Typ string          `json:"typ"`
	Alg string          `json:"alg"`
	JWK json.RawMessage `json:"jwk"`
}

type rawClaims struct {
	JTI string `json:"jti"`
	HTM string `json:"htm"`
	HTU string `json:"htu"`
	IAT int64  `json:"iat"`
}

type rawJWK struct {
	KTY string `json:"kty"`
	CRV string `json:"crv"`
	X   string `json:"x"`
	Y   string `json:"y"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// ParseAndValidate valida um DPoP proof JWT conforme RFC 9449.
// Retorna o jti (para registro anti-replay) ou erro.
func ParseAndValidate(proofJWT, expectedHTM, expectedHTU string) (string, error) {
	parts := strings.Split(proofJWT, ".")
	if len(parts) != 3 {
		return "", errors.New("dpop: formato JWT inválido")
	}

	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", fmt.Errorf("dpop: falha ao decodificar header: %w", err)
	}

	var hdr rawHeader
	if err := json.Unmarshal(headerBytes, &hdr); err != nil {
		return "", fmt.Errorf("dpop: falha ao parsear header: %w", err)
	}

	if hdr.Typ != "dpop+jwt" {
		return "", fmt.Errorf("dpop: typ inválido %q, esperado \"dpop+jwt\"", hdr.Typ)
	}
	if hdr.Alg != "ES256" && hdr.Alg != "RS256" {
		return "", fmt.Errorf("dpop: algoritmo %q não suportado", hdr.Alg)
	}
	if hdr.JWK == nil {
		return "", errors.New("dpop: jwk ausente no header")
	}

	var jwk rawJWK
	if err := json.Unmarshal(hdr.JWK, &jwk); err != nil {
		return "", fmt.Errorf("dpop: falha ao parsear jwk: %w", err)
	}

	var pubKey interface{}
	switch jwk.KTY {
	case "EC":
		pubKey, err = parseECKey(jwk)
	case "RSA":
		pubKey, err = parseRSAKey(jwk)
	default:
		err = fmt.Errorf("dpop: kty %q não suportado", jwk.KTY)
	}
	if err != nil {
		return "", err
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("dpop: falha ao decodificar payload: %w", err)
	}

	var claims rawClaims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return "", fmt.Errorf("dpop: falha ao parsear claims: %w", err)
	}

	if claims.JTI == "" {
		return "", errors.New("dpop: jti ausente")
	}

	if !strings.EqualFold(claims.HTM, expectedHTM) {
		return "", fmt.Errorf("dpop: htm %q inválido, esperado %q", claims.HTM, expectedHTM)
	}

	// Comparar htu sem query string nem fragmento
	htu := strings.SplitN(claims.HTU, "?", 2)[0]
	htu = strings.SplitN(htu, "#", 2)[0]

	// Decodifica ambos para evitar problemas de URL encoding (%20 vs espaço, etc.)
	if unescapedHTU, err := url.PathUnescape(htu); err == nil {
		htu = unescapedHTU
	}
	if unescapedExpected, err := url.PathUnescape(expectedHTU); err == nil {
		expectedHTU = unescapedExpected
	}

	if htu != expectedHTU {
		return "", fmt.Errorf("dpop: htu %q inválido, esperado %q", claims.HTU, expectedHTU)
	}

	now := time.Now().Unix()
	diff := claims.IAT - now
	if diff < 0 {
		diff = -diff
	}
	if diff > 60 {
		return "", errors.New("dpop: iat fora da janela de 60 segundos")
	}

	sigBytes, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return "", fmt.Errorf("dpop: falha ao decodificar assinatura: %w", err)
	}

	signingInput := parts[0] + "." + parts[1]
	if err := verifySignature(hdr.Alg, pubKey, signingInput, sigBytes); err != nil {
		return "", err
	}

	return claims.JTI, nil
}

func parseECKey(jwk rawJWK) (*ecdsa.PublicKey, error) {
	if jwk.CRV != "P-256" {
		return nil, fmt.Errorf("dpop: curva EC %q não suportada, esperado P-256", jwk.CRV)
	}

	xBytes, err := base64.RawURLEncoding.DecodeString(jwk.X)
	if err != nil {
		return nil, fmt.Errorf("dpop: falha ao decodificar coordenada X: %w", err)
	}
	yBytes, err := base64.RawURLEncoding.DecodeString(jwk.Y)
	if err != nil {
		return nil, fmt.Errorf("dpop: falha ao decodificar coordenada Y: %w", err)
	}

	curve := elliptic.P256()
	pub := &ecdsa.PublicKey{
		Curve: curve,
		X:     new(big.Int).SetBytes(xBytes),
		Y:     new(big.Int).SetBytes(yBytes),
	}

	if !curve.IsOnCurve(pub.X, pub.Y) {
		return nil, errors.New("dpop: ponto EC não está na curva P-256")
	}

	return pub, nil
}

func parseRSAKey(jwk rawJWK) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(jwk.N)
	if err != nil {
		return nil, fmt.Errorf("dpop: falha ao decodificar módulo RSA N: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(jwk.E)
	if err != nil {
		return nil, fmt.Errorf("dpop: falha ao decodificar expoente RSA E: %w", err)
	}

	eInt := int(new(big.Int).SetBytes(eBytes).Int64())
	if eInt == 0 {
		return nil, errors.New("dpop: expoente RSA E inválido")
	}

	pub := &rsa.PublicKey{
		N: new(big.Int).SetBytes(nBytes),
		E: eInt,
	}

	if pub.N.BitLen() < 2048 {
		return nil, errors.New("dpop: chave RSA menor que 2048 bits rejeitada")
	}

	return pub, nil
}

func verifySignature(alg string, pubKey interface{}, signingInput string, sigBytes []byte) error {
	h := crypto.SHA256.New()
	h.Write([]byte(signingInput))
	digest := h.Sum(nil)

	switch alg {
	case "ES256":
		ecKey, ok := pubKey.(*ecdsa.PublicKey)
		if !ok {
			return errors.New("dpop: tipo de chave incompatível com ES256")
		}
		if len(sigBytes) != 64 {
			return fmt.Errorf("dpop: assinatura ES256 deve ter 64 bytes, recebeu %d", len(sigBytes))
		}
		r := new(big.Int).SetBytes(sigBytes[:32])
		s := new(big.Int).SetBytes(sigBytes[32:])
		if !ecdsa.Verify(ecKey, digest, r, s) {
			return errors.New("dpop: assinatura ES256 inválida")
		}

	case "RS256":
		rsaKey, ok := pubKey.(*rsa.PublicKey)
		if !ok {
			return errors.New("dpop: tipo de chave incompatível com RS256")
		}
		if err := rsa.VerifyPKCS1v15(rsaKey, crypto.SHA256, digest, sigBytes); err != nil {
			return errors.New("dpop: assinatura RS256 inválida")
		}

	default:
		return fmt.Errorf("dpop: algoritmo %q não suportado na verificação", alg)
	}

	return nil
}

package auth

import (
	"errors"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// ExtractSubFromToken extrai o campo 'sub' do JWT sem validar a assinatura.
// Útil quando a validação da assinatura já foi feita por um gateway ou quando
// queremos apenas identificar o usuário para busca no banco.
func ExtractSubFromToken(authHeader string) (string, error) {
	if authHeader == "" {
		return "", errors.New("header Authorization ausente")
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || (parts[0] != "Bearer" && parts[0] != "DPoP") {
		return "", errors.New("formato de token inválido")
	}

	tokenString := parts[1]
	
	// Parse sem validar assinatura (Keycloak/APISIX validam a assinatura antes)
	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, jwt.MapClaims{})
	if err != nil {
		return "", err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok {
		if sub, ok := claims["sub"].(string); ok {
			return sub, nil
		}
	}

	return "", errors.New("campo 'sub' não encontrado no token")
}

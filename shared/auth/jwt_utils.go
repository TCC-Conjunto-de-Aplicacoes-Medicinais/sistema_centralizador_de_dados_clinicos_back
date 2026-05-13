package auth

import (
	"errors"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// UserClaims contém informações básicas extraídas do Access Token.
type UserClaims struct {
	PatientID     string `json:"sub"`
	Name          string `json:"name"`
	EmailVerified bool   `json:"email_verified"`
	Email         string `json:"email"`
}

// ExtractUserClaims extrai claims úteis do JWT sem validar a assinatura.
// A validação deve ser feita previamente por um middleware de segurança.
func ExtractUserClaims(authHeader string) (*UserClaims, error) {
	if authHeader == "" {
		return nil, errors.New("header Authorization ausente")
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || (parts[0] != "Bearer" && parts[0] != "DPoP") {
		return nil, errors.New("formato de token inválido")
	}

	tokenString := parts[1]
	
	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, jwt.MapClaims{})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("não foi possível ler as claims do token")
	}

	userClaims := &UserClaims{}

	if sub, ok := claims["sub"].(string); ok {
		userClaims.PatientID = sub
	} else {
		return nil, errors.New("campo 'sub' (ID do usuário) não encontrado no token")
	}

	if name, ok := claims["name"].(string); ok {
		userClaims.Name = name
	}

	if verified, ok := claims["email_verified"].(bool); ok {
		userClaims.EmailVerified = verified
	}

	if email, ok := claims["email"].(string); ok {
		userClaims.Email = email
	}

	return userClaims, nil
}


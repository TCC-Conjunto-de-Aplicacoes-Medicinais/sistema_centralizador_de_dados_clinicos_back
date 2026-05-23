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
	FullName      string `json:"full_name"`
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
		keys := []string{}
		for k := range claims {
			keys = append(keys, k)
		}
		return nil, errors.New("campo 'sub' não encontrado. Campos presentes: " + strings.Join(keys, ", "))
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

// ClinicClaims representa as claims para tokens de clinicas/médicos.
type ClinicClaims struct {
	ClinicID       string `json:"clinic_id"`
	ClinicalUserID uint   `json:"clinical_user_id"`
	Email          string `json:"email"`
	FullName       string `json:"full_name"`
	Role           string `json:"role"`
	jwt.RegisteredClaims
}

// GenerateClinicToken gera um token assinado para clinicas.
func GenerateClinicToken(claims ClinicClaims, secret string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// ValidateClinicToken valida o token assinado de clinicas.
func ValidateClinicToken(tokenStr string, secret string) (*ClinicClaims, error) {
	// Remove Bearer prefix se existir
	if strings.HasPrefix(tokenStr, "Bearer ") {
		tokenStr = strings.TrimPrefix(tokenStr, "Bearer ")
	}

	token, err := jwt.ParseWithClaims(tokenStr, &ClinicClaims{}, func(t *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*ClinicClaims)
	if !ok || !token.Valid {
		return nil, errors.New("token inválido")
	}
	return claims, nil
}



package config

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/Nerzal/gocloak/v13"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/auth"
)

type KeycloakAuth struct {
	Client       *gocloak.GoCloak
	ClientID     string
	ClientSecret string
	Realm        string
}

func InitKeycloak(url, clientID, secret, realm string) *KeycloakAuth {
	client := gocloak.NewClient(url)

	ctx := context.Background()

	_, err := client.LoginClient(ctx, clientID, secret, realm)
	if err != nil {
		log.Fatalf("❌ Erro ao conectar no Keycloak: %v", err)
	}

	log.Println("✅ Conexão com Keycloak estabelecida com sucesso!")

	return &KeycloakAuth{
		Client:       client,
		ClientID:     clientID,
		ClientSecret: secret,
		Realm:        realm,
	}
}

// VerifyAccessToken valida o access token junto ao Keycloak (RFC 7662) e devolve as claims.
// Rejeita tokens com assinatura inválida, expirados ou revogados. As claims só são lidas
// depois da introspecção bem-sucedida.
func (k *KeycloakAuth) VerifyAccessToken(ctx context.Context, token string) (map[string]interface{}, error) {
	res, err := k.Client.RetrospectToken(ctx, token, k.ClientID, k.ClientSecret, k.Realm)
	if err != nil {
		return nil, fmt.Errorf("falha na introspecção do token: %w", err)
	}
	if res.Active == nil || !*res.Active {
		return nil, errors.New("token inativo, expirado ou revogado")
	}
	return auth.UnverifiedClaims(token)
}

// IntrospectToken valida o token sem precisar das claims.
func (k *KeycloakAuth) IntrospectToken(ctx context.Context, token string) error {
	_, err := k.VerifyAccessToken(ctx, token)
	return err
}

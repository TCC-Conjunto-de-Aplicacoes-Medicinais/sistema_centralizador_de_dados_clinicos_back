package config

import (
	"log"
	"context"
	"github.com/Nerzal/gocloak/v13"
)

type KeycloakAuth struct {
	Client 		 *gocloak.GoCloak
	ClientID 	 string
	ClientSecret string
	Realm 		 string
}

func InitKeycloak(url, clientID, secret, realm string) *KeycloakAuth {
	client := gocloak.NewClient(url)
	
	ctx := context.Background()

	// Testamos a conexão tentando pegar um token via credenciais da aplicação (App Credentials)
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
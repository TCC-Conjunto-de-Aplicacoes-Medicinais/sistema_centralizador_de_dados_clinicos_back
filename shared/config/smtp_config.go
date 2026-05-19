package config

import (
	"log"
)

type SMTPConfig struct {
	Host     string
	Port     int
	User     string
	Password string
}

func InitSMTP(host string, port int, user, password string) *SMTPConfig {
	if host == "" || user == "" || password == "" {
		log.Println("⚠️ Atenção: Configurações de SMTP incompletas. O disparo de e-mail pode falhar.")
	}

	return &SMTPConfig{
		Host:     host,
		Port:     port,
		User:     user,
		Password: password,
	}
}

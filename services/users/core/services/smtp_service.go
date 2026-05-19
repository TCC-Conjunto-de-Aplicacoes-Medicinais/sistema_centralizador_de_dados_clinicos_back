package services

import (
	"context"
	"fmt"
	"net/smtp"

	sharedConfig "github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/config"
)

type SmtpEmailService struct {
	Config *sharedConfig.SMTPConfig
}

func NewSMTPEmailService(config *sharedConfig.SMTPConfig) *SmtpEmailService {
	return &SmtpEmailService{
		Config: config,
	}
}

func (s *SmtpEmailService) SendVerificationCode(ctx context.Context, toEmail, code string) error {
	auth := smtp.PlainAuth("", s.Config.User, s.Config.Password, s.Config.Host)

	subject := "Open Health - Seu Código de Verificação"
	mime := "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\n\n"

	htmlBody := fmt.Sprintf(`
	<!DOCTYPE html>
	<html>
	<head>
		<meta charset="UTF-8">
		<style>
			body { font-family: 'Inter', Arial, sans-serif; background-color: #F8FAF9; margin: 0; padding: 40px 0; }
			.container { max-width: 600px; margin: 0 auto; background: #ffffff; border-radius: 12px; overflow: hidden; box-shadow: 0 4px 20px rgba(0,0,0,0.05); }
			.header { background: linear-gradient(135deg, #00C853, #1B5E3B); padding: 30px; text-align: center; color: white; }
			.content { padding: 40px 30px; text-align: center; color: #333; }
			.code-box { background: #E0F2E7; padding: 20px; font-size: 32px; font-weight: 800; letter-spacing: 6px; color: #00C853; border-radius: 8px; margin: 30px 0; display: inline-block; }
			.footer { padding: 20px; text-align: center; font-size: 12px; color: #888; background: #f9f9f9; }
		</style>
	</head>
	<body>
		<div class="container">
			<div class="header">
				<h1 style="margin: 0; font-size: 24px;">Bem-vindo à Open Health!</h1>
			</div>
			<div class="content">
				<p style="font-size: 16px; margin-bottom: 10px;">Para garantir a segurança da sua conta e do acesso aos seus dados de saúde,</p>
				<p style="font-size: 16px;">copie o código abaixo e cole na tela de verificação:</p>
				
				<div class="code-box">%s</div>
				
				<p style="font-size: 14px; color: #666;">Se você não solicitou este cadastro, pode ignorar este e-mail.</p>
			</div>
			<div class="footer">
				&copy; 2026 Open Health. Todos os direitos reservados.
			</div>
		</div>
	</body>
	</html>
	`, code)

	msg := []byte(fmt.Sprintf("To: %s\r\nSubject: %s\r\n%s%s", toEmail, subject, mime, htmlBody))

	addr := fmt.Sprintf("%s:%d", s.Config.Host, s.Config.Port)
	return smtp.SendMail(addr, auth, s.Config.User, []string{toEmail}, msg)
}

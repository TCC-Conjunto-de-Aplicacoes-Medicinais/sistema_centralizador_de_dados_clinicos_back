package services

import (
	"context"
	"fmt"
	"net/smtp"
	"time"

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

func (s *SmtpEmailService) SendEmergencyAccessAlert(ctx context.Context, toEmail, clinicName, requesterDetails, justification string) error {
	auth := smtp.PlainAuth("", s.Config.User, s.Config.Password, s.Config.Host)

	subject := "ALERTA: Acesso Crítico aos seus Dados de Saúde (Break the Glass)"
	mime := "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\n\n"

	htmlBody := fmt.Sprintf(`
	<!DOCTYPE html>
	<html>
	<head>
		<meta charset="UTF-8">
		<style>
			body { font-family: Arial, sans-serif; background-color: #FFF0F0; margin: 0; padding: 40px 0; }
			.container { max-width: 600px; margin: 0 auto; background: #ffffff; border-radius: 12px; overflow: hidden; box-shadow: 0 4px 20px rgba(0,0,0,0.1); border: 2px solid #E53935; }
			.header { background: linear-gradient(135deg, #E53935, #B71C1C); padding: 30px; text-align: center; color: white; }
			.content { padding: 40px 30px; color: #333; line-height: 1.6; }
			.alert-box { background: #FFEBEE; border-left: 5px solid #E53935; padding: 15px; margin: 20px 0; border-radius: 4px; }
			.footer { padding: 20px; text-align: center; font-size: 12px; color: #888; background: #f9f9f9; }
		</style>
	</head>
	<body>
		<div class="container">
			<div class="header">
				<h1 style="margin: 0; font-size: 22px;">Acesso de Emergência Registrado</h1>
			</div>
			<div class="content">
				<p style="font-size: 16px;">Olá,</p>
				<p style="font-size: 16px;">Informamos que seus dados clínicos centralizados foram acessados em caráter de emergência através do protocolo <strong>"Break the Glass"</strong>.</p>
				
				<div class="alert-box">
					<p style="margin: 5px 0;"><strong>Clínica Requisitante:</strong> %s</p>
					<p style="margin: 5px 0;"><strong>Profissional:</strong> %s</p>
					<p style="margin: 5px 0;"><strong>Justificativa Médica:</strong> %s</p>
					<p style="margin: 5px 0;"><strong>Data/Hora:</strong> %s</p>
				</div>
				
				<p style="font-size: 14px; color: #555;">Este procedimento é previsto por lei para situações de emergência ou risco iminente à vida, onde não foi possível obter o código de autorização OTP temporário.</p>
				<p style="font-size: 14px; color: #666; font-style: italic;">Se você não reconhece esta situação ou acredita que houve uso indevido, entre em contato com o suporte da Open Health imediatamente.</p>
			</div>
			<div class="footer">
				&copy; 2026 Open Health. Todos os direitos reservados.
			</div>
		</div>
	</body>
	</html>
	`, clinicName, requesterDetails, justification, time.Now().Format("02/01/2006 15:04:05"))

	msg := []byte(fmt.Sprintf("To: %s\r\nSubject: %s\r\n%s%s", toEmail, subject, mime, htmlBody))

	addr := fmt.Sprintf("%s:%d", s.Config.Host, s.Config.Port)
	return smtp.SendMail(addr, auth, s.Config.User, []string{toEmail}, msg)
}


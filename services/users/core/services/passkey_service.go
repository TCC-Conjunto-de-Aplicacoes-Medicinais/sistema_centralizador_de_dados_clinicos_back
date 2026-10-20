package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/database"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/gocql/gocql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrNoPasskey        = errors.New("cadastre uma passkey neste aparelho antes de autorizar")
	ErrChallengeInvalid = errors.New("desafio de passkey inválido ou expirado")
	ErrRecoveryInvalid  = errors.New("código de recuperação inválido ou expirado")
)

const (
	passkeyChallengeTTL = 5 * time.Minute
	recoveryTTL         = 10 * time.Minute
	recoveryMaxAttempts = 5

	purposeRegister = "register"
	purposeAction   = "action"
)

// passkeyChallenge é um desafio WebAuthn pendente. Ele é de uso único e está preso a um paciente,
// a um propósito e, para ações, à ação exata (ex.: "consent:grant:clinical_history_read").
type passkeyChallenge struct {
	session   webauthn.SessionData
	patientID string
	purpose   string
	action    string
	expires   time.Time
}

// PasskeyService cadastra passkeys do paciente e verifica as assinaturas que autorizam consentimentos.
// shortcut: desafios ficam em memória, então só valem para uma instância e se perdem ao reiniciar; migrar para Cassandra ou Redis ao escalar.
type PasskeyService struct {
	DB   *gorm.DB
	WA   *webauthn.WebAuthn
	SMTP *SmtpEmailService

	mu         sync.Mutex
	challenges map[string]passkeyChallenge
}

// NewPasskeyService aceita várias origens: web (https://app...), iOS (https://rpID) e Android (android:apk-key-hash:...).
func NewPasskeyService(db *gorm.DB, smtp *SmtpEmailService, rpID string, origins []string) (*PasskeyService, error) {
	wa, err := webauthn.New(&webauthn.Config{RPID: rpID, RPDisplayName: "POHINC", RPOrigins: origins})
	if err != nil {
		return nil, fmt.Errorf("configuração WebAuthn inválida: %w", err)
	}
	return &PasskeyService{DB: db, WA: wa, SMTP: smtp, challenges: map[string]passkeyChallenge{}}, nil
}

// passkeyUser adapta o paciente para a interface do WebAuthn.
type passkeyUser struct {
	id    string
	creds []webauthn.Credential
}

func (u passkeyUser) WebAuthnID() []byte                         { return []byte(u.id) }
func (u passkeyUser) WebAuthnName() string                       { return u.id }
func (u passkeyUser) WebAuthnDisplayName() string                { return "Paciente POHINC" }
func (u passkeyUser) WebAuthnCredentials() []webauthn.Credential { return u.creds }

func (s *PasskeyService) loadUser(ctx context.Context, patientID string) (passkeyUser, error) {
	var rows []database.PatientPasskey
	if err := s.DB.WithContext(ctx).Where("patient_id = ?", patientID).Find(&rows).Error; err != nil {
		return passkeyUser{}, fmt.Errorf("falha ao ler passkeys: %w", err)
	}
	u := passkeyUser{id: patientID}
	for _, r := range rows {
		var cred webauthn.Credential
		if err := json.Unmarshal([]byte(r.CredentialJSON), &cred); err != nil {
			return passkeyUser{}, fmt.Errorf("credencial corrompida: %w", err)
		}
		u.creds = append(u.creds, cred)
	}
	return u, nil
}

// BeginRegistration gera as opções para o navegador criar uma passkey nova para o paciente.
func (s *PasskeyService) BeginRegistration(ctx context.Context, patientID string) (*protocol.CredentialCreation, string, error) {
	u, err := s.loadUser(ctx, patientID)
	if err != nil {
		return nil, "", err
	}
	creation, session, err := s.WA.BeginRegistration(u,
		webauthn.WithExclusions(webauthn.Credentials(u.creds).CredentialDescriptors()),
		webauthn.WithAuthenticatorSelection(protocol.AuthenticatorSelection{UserVerification: protocol.VerificationRequired}),
	)
	if err != nil {
		return nil, "", fmt.Errorf("falha ao iniciar cadastro da passkey: %w", err)
	}
	id, err := s.storeChallenge(passkeyChallenge{session: *session, patientID: patientID, purpose: purposeRegister})
	return creation, id, err
}

// FinishRegistration valida a resposta do navegador e grava a passkey do paciente.
func (s *PasskeyService) FinishRegistration(ctx context.Context, patientID, challengeID string, r *http.Request) error {
	c, err := s.takeChallenge(challengeID, patientID, purposeRegister, "")
	if err != nil {
		return err
	}
	u, err := s.loadUser(ctx, patientID)
	if err != nil {
		return err
	}
	cred, err := s.WA.FinishRegistration(u, c.session, r)
	if err != nil {
		return fmt.Errorf("passkey recusada: %w", err)
	}
	raw, err := json.Marshal(cred)
	if err != nil {
		return err
	}
	return s.DB.WithContext(ctx).Create(&database.PatientPasskey{
		ID:             gocql.TimeUUID().String(),
		PatientID:      patientID,
		CredentialID:   encodeCredentialID(cred.ID),
		CredentialJSON: string(raw),
	}).Error
}

// BeginAction gera o desafio que o paciente precisa assinar com a passkey para autorizar uma ação.
func (s *PasskeyService) BeginAction(ctx context.Context, patientID, action string) (*protocol.CredentialAssertion, string, error) {
	u, err := s.loadUser(ctx, patientID)
	if err != nil {
		return nil, "", err
	}
	if len(u.creds) == 0 {
		return nil, "", ErrNoPasskey
	}
	assertion, session, err := s.WA.BeginLogin(u, webauthn.WithUserVerification(protocol.VerificationRequired))
	if err != nil {
		return nil, "", fmt.Errorf("falha ao iniciar assinatura: %w", err)
	}
	id, err := s.storeChallenge(passkeyChallenge{session: *session, patientID: patientID, purpose: purposeAction, action: action})
	return assertion, id, err
}

// VerifyAction confere a assinatura da passkey para a ação indicada e atualiza o contador da credencial.
func (s *PasskeyService) VerifyAction(ctx context.Context, patientID, challengeID, action string, r *http.Request) error {
	c, err := s.takeChallenge(challengeID, patientID, purposeAction, action)
	if err != nil {
		return err
	}
	u, err := s.loadUser(ctx, patientID)
	if err != nil {
		return err
	}
	cred, err := s.WA.FinishLogin(u, c.session, r)
	if err != nil {
		return fmt.Errorf("assinatura recusada: %w", err)
	}
	if cred.Authenticator.CloneWarning {
		return errors.New("assinatura recusada: possível clonagem da passkey")
	}
	raw, err := json.Marshal(cred)
	if err != nil {
		return err
	}
	now := time.Now()
	return s.DB.WithContext(ctx).Model(&database.PatientPasskey{}).
		Where("credential_id = ?", encodeCredentialID(cred.ID)).
		Updates(map[string]interface{}{"credential_json": string(raw), "last_used_at": now}).Error
}

// RequestRecoveryCode envia um código de 6 dígitos ao e-mail. Para e-mail desconhecido a resposta é a mesma (sem enumeração).
func (s *PasskeyService) RequestRecoveryCode(ctx context.Context, email string) error {
	patientID, err := s.patientIDByEmail(ctx, email)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return err
	}
	code := fmt.Sprintf("%06d", n.Int64())
	expires := time.Now().Add(recoveryTTL)
	rec := database.PatientRecovery{PatientID: patientID, CodeHash: ptr(hashSecret(code)), CodeExpiresAt: &expires}
	if err := s.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "patient_id"}},
		UpdateAll: true,
	}).Create(&rec).Error; err != nil {
		return fmt.Errorf("falha ao gravar código de recuperação: %w", err)
	}
	return s.SMTP.SendVerificationCode(ctx, email, code)
}

// VerifyRecoveryCode confere o código e devolve o token de uso único para cadastrar uma passkey nova.
func (s *PasskeyService) VerifyRecoveryCode(ctx context.Context, email, code string) (string, error) {
	patientID, err := s.patientIDByEmail(ctx, email)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", ErrRecoveryInvalid
	}
	if err != nil {
		return "", err
	}
	var rec database.PatientRecovery
	if err := s.DB.WithContext(ctx).Where("patient_id = ?", patientID).Take(&rec).Error; err != nil {
		return "", ErrRecoveryInvalid
	}
	if rec.CodeHash == nil || rec.CodeExpiresAt == nil || time.Now().After(*rec.CodeExpiresAt) || rec.CodeAttempts >= recoveryMaxAttempts {
		return "", ErrRecoveryInvalid
	}
	// A tentativa é contada antes da comparação, para que um código não seja testado em paralelo.
	if err := s.DB.WithContext(ctx).Model(&rec).Update("code_attempts", gorm.Expr("code_attempts + 1")).Error; err != nil {
		return "", err
	}
	if subtle.ConstantTimeCompare([]byte(hashSecret(code)), []byte(*rec.CodeHash)) != 1 {
		return "", ErrRecoveryInvalid
	}
	token, err := randomSecret()
	if err != nil {
		return "", err
	}
	expires := time.Now().Add(recoveryTTL)
	if err := s.DB.WithContext(ctx).Model(&database.PatientRecovery{}).Where("patient_id = ?", patientID).Updates(map[string]interface{}{
		"code_hash":        nil,
		"code_expires_at":  nil,
		"code_attempts":    0,
		"token_hash":       hashSecret(token),
		"token_expires_at": expires,
	}).Error; err != nil {
		return "", err
	}
	return token, nil
}

// BeginRecoveryRegistration gera o desafio de cadastro de passkey usando o token de recuperação.
func (s *PasskeyService) BeginRecoveryRegistration(ctx context.Context, token string) (*protocol.CredentialCreation, string, error) {
	patientID, err := s.patientForRecoveryToken(ctx, token)
	if err != nil {
		return nil, "", err
	}
	return s.BeginRegistration(ctx, patientID)
}

// FinishRecoveryRegistration grava a passkey nova e encerra o token de recuperação.
func (s *PasskeyService) FinishRecoveryRegistration(ctx context.Context, token, challengeID string, r *http.Request) error {
	patientID, err := s.patientForRecoveryToken(ctx, token)
	if err != nil {
		return err
	}
	if err := s.FinishRegistration(ctx, patientID, challengeID, r); err != nil {
		return err
	}
	return s.DB.WithContext(ctx).Model(&database.PatientRecovery{}).Where("patient_id = ?", patientID).
		Updates(map[string]interface{}{"token_hash": nil, "token_expires_at": nil}).Error
}

func (s *PasskeyService) patientForRecoveryToken(ctx context.Context, token string) (string, error) {
	var rec database.PatientRecovery
	err := s.DB.WithContext(ctx).
		Where("token_hash = ? AND token_expires_at > ?", hashSecret(token), time.Now()).
		Take(&rec).Error
	if err != nil {
		return "", ErrRecoveryInvalid
	}
	return rec.PatientID, nil
}

func (s *PasskeyService) patientIDByEmail(ctx context.Context, email string) (string, error) {
	var pe database.PatientEmail
	err := s.DB.WithContext(ctx).
		Where("LOWER(email) = ?", strings.ToLower(strings.TrimSpace(email))).
		Order("principal DESC").
		First(&pe).Error
	return pe.PatientID, err
}

func (s *PasskeyService) storeChallenge(c passkeyChallenge) (string, error) {
	id, err := randomSecret()
	if err != nil {
		return "", err
	}
	c.expires = time.Now().Add(passkeyChallengeTTL)
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for k, v := range s.challenges {
		if now.After(v.expires) {
			delete(s.challenges, k)
		}
	}
	s.challenges[id] = c
	return id, nil
}

// takeChallenge consome o desafio (uso único, mesmo se a verificação falhar) e confere o dono e a ação.
func (s *PasskeyService) takeChallenge(id, patientID, purpose, action string) (passkeyChallenge, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.challenges[id]
	delete(s.challenges, id)
	if !ok || time.Now().After(c.expires) || c.patientID != patientID || c.purpose != purpose || c.action != action {
		return passkeyChallenge{}, ErrChallengeInvalid
	}
	return c, nil
}

func hashSecret(v string) string {
	sum := sha256.Sum256([]byte(v))
	return hex.EncodeToString(sum[:])
}

func randomSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func encodeCredentialID(id []byte) string {
	return base64.RawURLEncoding.EncodeToString(id)
}

func ptr[T any](v T) *T {
	return &v
}

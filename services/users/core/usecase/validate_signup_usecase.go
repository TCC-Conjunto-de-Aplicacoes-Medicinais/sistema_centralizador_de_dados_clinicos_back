package usecase

import (
	"context"
	"errors"
	"regexp"
	"strconv"
	"strings"

	sharedConfig "github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/config"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/database"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/models"

	"github.com/Nerzal/gocloak/v13"
	"gorm.io/gorm"
)

type ValidateSignupUseCase struct {
	DB       *gorm.DB
	Keycloak *sharedConfig.KeycloakAuth
}

func NewValidateSignupUseCase(db *gorm.DB, kc *sharedConfig.KeycloakAuth) *ValidateSignupUseCase {
	return &ValidateSignupUseCase{DB: db, Keycloak: kc}
}

func (uc *ValidateSignupUseCase) Execute(ctx context.Context, req models.SignupRequest) error {
	if !isValidCPF(req.CPF) {
		return errors.New("validação falhou: formato de CPF inválido")
	}

	if !isValidEmail(req.Email) {
		return errors.New("validação falhou: formato de e-mail inválido")
	}

	var count int64

	if err := uc.DB.Model(&database.Patients{}).Where("cpf = ?", req.CPF).Count(&count).Error; err != nil {
		return errors.New("erro ao verificar integridade do CPF no banco de dados")
	}
	if count > 0 {
		return errors.New("validação falhou: o CPF informado já está em uso")
	}

	if err := uc.DB.Model(&database.Patients{}).Where("email = ?", req.Email).Count(&count).Error; err != nil {
		return errors.New("erro ao verificar integridade do E-mail no banco de dados")
	}
	if count > 0 {
		return errors.New("validação falhou: o E-mail informado já está em uso")
	}

	token, err := uc.Keycloak.Client.LoginClient(ctx, uc.Keycloak.ClientID, uc.Keycloak.ClientSecret, uc.Keycloak.Realm)
	if err == nil {
		users, _ := uc.Keycloak.Client.GetUsers(ctx, token.AccessToken, uc.Keycloak.Realm, gocloak.GetUsersParams{
			Email: gocloak.StringP(req.Email),
			Exact: gocloak.BoolP(true),
		})

		if len(users) > 0 {
			return errors.New("validação falhou: uma conta de acesso vinculada a este e-mail já existe no Keycloak")
		}
	}

	return nil
}

func isValidCPF(cpf string) bool {
	cpf = strings.ReplaceAll(cpf, ".", "")
	cpf = strings.ReplaceAll(cpf, "-", "")
	if len(cpf) != 11 {
		return false
	}
	allSame := true
	for i := 1; i < 11; i++ {
		if cpf[i] != cpf[0] {
			allSame = false
			break
		}
	}
	if allSame {
		return false
	}
	sum := 0
	for i := 0; i < 9; i++ {
		num, _ := strconv.Atoi(string(cpf[i]))
		sum += num * (10 - i)
	}
	rem := (sum * 10) % 11
	if rem == 10 || rem == 11 {
		rem = 0
	}
	if strconv.Itoa(rem) != string(cpf[9]) {
		return false
	}
	sum = 0
	for i := 0; i < 10; i++ {
		num, _ := strconv.Atoi(string(cpf[i]))
		sum += num * (11 - i)
	}
	rem = (sum * 10) % 11
	if rem == 10 || rem == 11 {
		rem = 0
	}
	if strconv.Itoa(rem) != string(cpf[10]) {
		return false
	}
	return true
}

func isValidEmail(email string) bool {
	re := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	return re.MatchString(email)
}

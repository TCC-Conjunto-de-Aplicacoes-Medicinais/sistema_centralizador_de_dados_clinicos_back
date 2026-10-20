package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/database"
	"github.com/gocql/gocql"
	"gorm.io/gorm"
)

// ScopeClinicalRead é o único escopo de consentimento hoje: leitura do histórico federado por clínicas.
const ScopeClinicalRead = "clinical_history_read"

var (
	ErrNoConsent       = errors.New("consentimento do paciente ausente ou revogado")
	ErrPatientNotFound = errors.New("paciente não encontrado")
)

// ClinicalSyncService concentra a troca de dados entre clínicas federadas, o centralizador
// e o paciente. A clínica não acessa o banco diretamente: tudo passa por aqui.
type ClinicalSyncService struct {
	DB      *gorm.DB
	Records *ClinicalRecordStore
}

// NewClinicalSyncService: MariaDB guarda pacientes, consentimentos e exames; Cassandra guarda os prontuários FHIR.
func NewClinicalSyncService(db *gorm.DB, records *ClinicalRecordStore) *ClinicalSyncService {
	return &ClinicalSyncService{DB: db, Records: records}
}

// PatientSyncInput são os dados cadastrais que uma clínica envia ao centralizador.
type PatientSyncInput struct {
	Name      string
	CPF       string
	BirthDate string
	Email     string
	Phone     string
	Gender    string
}

// UpsertPatient cria ou atualiza o paciente identificado pelo CPF (chave única no centralizador).
func (s *ClinicalSyncService) UpsertPatient(ctx context.Context, in PatientSyncInput) (*database.Patient, error) {
	cpf := digitsOnly(in.CPF)
	if len(cpf) != 11 {
		return nil, errors.New("CPF inválido: informe 11 dígitos")
	}
	if strings.TrimSpace(in.Name) == "" {
		return nil, errors.New("nome do paciente é obrigatório")
	}
	var p database.Patient
	var birth time.Time
	var err error
	if strings.TrimSpace(in.BirthDate) != "" {
		b, err := parseBirthDate(in.BirthDate)
		if err != nil {
			return nil, err
		}
		birth = b
	}

	err = s.DB.WithContext(ctx).Where("cpf = ?", cpf).First(&p).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		p = database.Patient{
			Id:        gocql.TimeUUID().String(),
			Name:      in.Name,
			CPF:       cpf,
			BirthDate: birth,
			Gender:    in.Gender,
		}
		if in.Phone != "" {
			p.Phones = []database.PatientPhone{{Id: gocql.TimeUUID().String(), Phone: in.Phone, Principal: true}}
		}
		if in.Email != "" {
			p.Emails = []database.PatientEmail{{Id: gocql.TimeUUID().String(), Email: in.Email, Principal: true}}
		}
		if err := s.DB.WithContext(ctx).Create(&p).Error; err != nil {
			return nil, fmt.Errorf("falha ao criar paciente: %w", err)
		}
		return &p, nil
	case err != nil:
		return nil, fmt.Errorf("falha ao buscar paciente: %w", err)
	}

	updates := map[string]interface{}{"name": in.Name}
	if !birth.IsZero() {
		updates["birth_date"] = birth
	}
	if in.Gender != "" {
		updates["gender"] = in.Gender
	}
	if err := s.DB.WithContext(ctx).Model(&p).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("falha ao atualizar paciente: %w", err)
	}
	return &p, nil
}

// SearchPatients busca por CPF exato ou por parte do nome (no máximo 20 resultados).
func (s *ClinicalSyncService) SearchPatients(ctx context.Context, cpf, name string) ([]database.Patient, error) {
	q := s.DB.WithContext(ctx).Limit(20)
	if d := digitsOnly(cpf); d != "" {
		q = q.Where("cpf = ?", d)
	} else if n := strings.TrimSpace(name); n != "" {
		q = q.Where("name LIKE ?", "%"+n+"%")
	} else {
		return nil, errors.New("informe cpf ou name")
	}
	var list []database.Patient
	if err := q.Find(&list).Error; err != nil {
		return nil, fmt.Errorf("falha na busca de pacientes: %w", err)
	}
	return list, nil
}

// IngestResult é o que o centralizador devolve após gravar um prontuário enviado por uma clínica.
type IngestResult struct {
	PatientID string
	RecordID  string
}

// IngestDiagnosticReport grava um DiagnosticReport FHIR R4 enviado por uma clínica.
// Reenvio do mesmo (clínica, id FHIR) atualiza o registro existente.
func (s *ClinicalSyncService) IngestDiagnosticReport(ctx context.Context, report map[string]interface{}) (*IngestResult, error) {
	if str(report, "resourceType") != "DiagnosticReport" {
		return nil, errors.New("resourceType deve ser DiagnosticReport")
	}
	fhirID := str(report, "id")
	clinicID := str(report, "meta", "clinic_id")
	if fhirID == "" || clinicID == "" {
		return nil, errors.New("id do recurso e meta.clinic_id são obrigatórios")
	}

	cpf := str(report, "subject", "identifier", "value")
	if cpf == "" {
		cpf = str(report, "meta", "patient_cpf")
	}
	name := str(report, "meta", "patient_name")
	if name == "" {
		name = str(report, "subject", "display")
	}
	patient, err := s.UpsertPatient(ctx, PatientSyncInput{
		Name:      name,
		CPF:       cpf,
		BirthDate: str(report, "meta", "patient_birth_date"),
	})
	if err != nil {
		return nil, fmt.Errorf("paciente do prontuário inválido: %w", err)
	}

	effective := time.Now()
	if raw := str(report, "effectiveDateTime"); raw != "" {
		if t, err := time.Parse(time.RFC3339, raw); err == nil {
			effective = t
		}
	}
	raw, err := json.Marshal(report)
	if err != nil {
		return nil, fmt.Errorf("falha ao serializar FHIR: %w", err)
	}

	recordID, err := s.Records.Upsert(ctx, patient.Id, clinicID, fhirID, effective,
		str(report, "code", "text"), str(report, "conclusion"), string(raw))
	if err != nil {
		return nil, err
	}
	return &IngestResult{PatientID: patient.Id, RecordID: recordID}, nil
}

// HasActiveConsent indica se o paciente tem consentimento de leitura vigente.
func (s *ClinicalSyncService) HasActiveConsent(ctx context.Context, patientID string) (bool, error) {
	var n int64
	now := time.Now()
	err := s.DB.WithContext(ctx).Model(&database.PatientConsent{}).
		Where("patient_id = ? AND scope = ? AND granted_at <= ? AND revoked_at IS NULL AND (valid_until IS NULL OR valid_until > ?)",
			patientID, ScopeClinicalRead, now, now).
		Count(&n).Error
	return n > 0, err
}

// BuildPatientBundle monta um Bundle FHIR R4 do paciente. Exige consentimento vigente.
func (s *ClinicalSyncService) BuildPatientBundle(ctx context.Context, patientID string) (map[string]interface{}, error) {
	ok, err := s.HasActiveConsent(ctx, patientID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNoConsent
	}

	var p database.Patient
	if err := s.DB.WithContext(ctx).Where("id = ?", patientID).First(&p).Error; err != nil {
		return nil, ErrPatientNotFound
	}

	records, err := s.Records.ListByPatient(ctx, patientID)
	if err != nil {
		return nil, fmt.Errorf("falha ao buscar prontuários: %w", err)
	}

	entries := []map[string]interface{}{{"resource": patientResource(p)}}

	// Exames enviados pelo próprio paciente (app) também fazem parte do histórico compartilhado.
	var uploads []database.Exam
	if err := s.DB.WithContext(ctx).Where("patient_id = ? AND flag_active = ?", patientID, true).
		Order("date desc").Find(&uploads).Error; err != nil {
		return nil, fmt.Errorf("falha ao buscar exames do paciente: %w", err)
	}
	for _, e := range uploads {
		entries = append(entries, map[string]interface{}{"resource": examResource(e)})
	}
	for _, rec := range records {
		var res map[string]interface{}
		if err := json.Unmarshal([]byte(rec.FHIRJSON), &res); err != nil {
			continue
		}
		entries = append(entries, map[string]interface{}{"resource": res})
	}

	return map[string]interface{}{
		"resourceType": "Bundle",
		"type":         "searchset",
		"total":        len(entries),
		"entry":        entries,
	}, nil
}

// PatientIDByKeycloakID resolve o paciente autenticado (claim sub do Keycloak).
func (s *ClinicalSyncService) PatientIDByKeycloakID(ctx context.Context, keycloakID string) (string, error) {
	var p database.Patient
	if err := s.DB.WithContext(ctx).Select("id").Where("keycloak_id = ?", keycloakID).First(&p).Error; err != nil {
		return "", ErrPatientNotFound
	}
	return p.Id, nil
}

// GrantConsent registra o consentimento do paciente para o escopo informado.
func (s *ClinicalSyncService) GrantConsent(ctx context.Context, patientID, scope string, validUntil *time.Time) (*database.PatientConsent, error) {
	if scope != ScopeClinicalRead {
		return nil, fmt.Errorf("escopo desconhecido: %s", scope)
	}
	c := database.PatientConsent{
		ID:         gocql.TimeUUID().String(),
		PatientID:  patientID,
		Scope:      scope,
		GrantedAt:  time.Now(),
		ValidUntil: validUntil,
	}
	if err := s.DB.WithContext(ctx).Create(&c).Error; err != nil {
		return nil, fmt.Errorf("falha ao registrar consentimento: %w", err)
	}
	return &c, nil
}

// RevokeConsent revoga um consentimento do próprio paciente.
func (s *ClinicalSyncService) RevokeConsent(ctx context.Context, patientID, consentID string) error {
	res := s.DB.WithContext(ctx).Model(&database.PatientConsent{}).
		Where("id = ? AND patient_id = ? AND revoked_at IS NULL", consentID, patientID).
		Update("revoked_at", time.Now())
	if res.Error != nil {
		return fmt.Errorf("falha ao revogar consentimento: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return errors.New("consentimento não encontrado ou já revogado")
	}
	return nil
}

// ListConsents lista os consentimentos do paciente, do mais recente para o mais antigo.
func (s *ClinicalSyncService) ListConsents(ctx context.Context, patientID string) ([]database.PatientConsent, error) {
	var list []database.PatientConsent
	err := s.DB.WithContext(ctx).Where("patient_id = ?", patientID).Order("granted_at desc").Find(&list).Error
	return list, err
}

// examResource converte um exame enviado pelo paciente em DiagnosticReport FHIR R4.
func examResource(e database.Exam) map[string]interface{} {
	conclusion := ""
	if e.ExamResult != nil {
		conclusion = *e.ExamResult
	}
	performer := ""
	if e.Institution != nil {
		performer = *e.Institution
	}
	return map[string]interface{}{
		"resourceType":      "DiagnosticReport",
		"id":                e.Id,
		"status":            "final",
		"code":              map[string]interface{}{"text": e.ExamType},
		"effectiveDateTime": e.Date.Format(time.RFC3339),
		"conclusion":        conclusion,
		"performer":         []map[string]string{{"display": performer}},
		"presentedForm":     []map[string]string{{"url": e.LinkBucket}},
		"meta":              map[string]string{"source": "paciente_upload"},
	}
}

func patientResource(p database.Patient) map[string]interface{} {
	return map[string]interface{}{
		"resourceType": "Patient",
		"id":           p.Id,
		"identifier":   []map[string]string{{"system": "urn:oid:2.16.76.4.1", "value": p.CPF}},
		"name":         []map[string]string{{"text": p.Name}},
		"birthDate":    p.BirthDate.Format("2006-01-02"),
		"gender":       p.Gender,
	}
}

func parseBirthDate(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	// Datas são gravadas em meia-noite local: o DSN usa loc=Local, então UTC puro deslocaria o dia.
	if t, err := time.ParseInLocation("2006-01-02", raw, time.Local); err == nil {
		return t, nil
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		y, m, d := t.Date()
		return time.Date(y, m, d, 0, 0, 0, 0, time.Local), nil
	}
	return time.Time{}, errors.New("data de nascimento inválida (use AAAA-MM-DD)")
}

func digitsOnly(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// str lê um campo string de um objeto JSON aninhado; retorna "" se ausente.
func str(m map[string]interface{}, path ...string) string {
	cur := m
	for i, key := range path {
		if i == len(path)-1 {
			v, _ := cur[key].(string)
			return strings.TrimSpace(v)
		}
		next, ok := cur[key].(map[string]interface{})
		if !ok {
			return ""
		}
		cur = next
	}
	return ""
}

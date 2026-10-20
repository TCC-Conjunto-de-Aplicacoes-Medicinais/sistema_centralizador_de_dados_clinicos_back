package services

import (
	"fmt"
	"time"

	"github.com/gocql/gocql"
)

// AccessAuditService grava no Cassandra quem acessou dados de qual paciente (LGPD: rastreabilidade).
// Duas tabelas, para responder às duas perguntas: "o que este usuário acessou?" e
// "quem acessou os dados deste paciente?".
type AccessAuditService struct {
	Session *gocql.Session
}

func NewAccessAuditService(session *gocql.Session) *AccessAuditService {
	return &AccessAuditService{Session: session}
}

// AuditEntry é um registro de acesso devolvido para consulta.
type AuditEntry struct {
	Actor     string    `json:"actor"`
	Action    string    `json:"action"`
	PatientID string    `json:"patient_id,omitempty"`
	Result    string    `json:"result"`
	At        time.Time `json:"at"`
}

// Record grava o acesso nas duas tabelas. Falha aqui deve bloquear o acesso (fail closed):
// sem registro, não há rastreabilidade.
func (s *AccessAuditService) Record(actor, action, patientID, result string) error {
	eventTime := gocql.TimeUUID()
	if err := s.Session.Query(
		`INSERT INTO access_audit_by_actor (actor, event_time, action, patient_id, result) VALUES (?, ?, ?, ?, ?)`,
		actor, eventTime, action, patientID, result,
	).Exec(); err != nil {
		return fmt.Errorf("falha ao gravar auditoria (ator): %w", err)
	}
	if patientID != "" {
		if err := s.Session.Query(
			`INSERT INTO access_audit_by_patient (patient_id, event_time, actor, action, result) VALUES (?, ?, ?, ?, ?)`,
			patientID, eventTime, actor, action, result,
		).Exec(); err != nil {
			return fmt.Errorf("falha ao gravar auditoria (paciente): %w", err)
		}
	}
	return nil
}

// ListByPatient devolve os acessos aos dados de um paciente, do mais recente para o mais antigo.
func (s *AccessAuditService) ListByPatient(patientID string, limit int) ([]AuditEntry, error) {
	iter := s.Session.Query(
		`SELECT actor, action, result, event_time FROM access_audit_by_patient WHERE patient_id = ? LIMIT ?`,
		patientID, limit,
	).Iter()
	var out []AuditEntry
	var actor, action, result string
	var eventTime gocql.UUID
	for iter.Scan(&actor, &action, &result, &eventTime) {
		out = append(out, AuditEntry{
			Actor:     actor,
			Action:    action,
			PatientID: patientID,
			Result:    result,
			At:        eventTime.Time(),
		})
	}
	if err := iter.Close(); err != nil {
		return nil, fmt.Errorf("falha ao ler auditoria: %w", err)
	}
	return out, nil
}

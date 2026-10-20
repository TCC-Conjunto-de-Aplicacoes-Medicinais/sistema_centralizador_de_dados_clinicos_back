package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/gocql/gocql"
)

// ClinicalRecordStore guarda os recursos FHIR (DiagnosticReport) das clínicas no Cassandra.
// Duas tabelas, cada uma para um acesso:
//   - clinical_record_by_patient: histórico do paciente (bundle), ordenado por data.
//   - clinical_record_by_origin: localiza um registro pela clínica de origem e pelo id FHIR,
//     para que o reenvio atualize em vez de duplicar.
type ClinicalRecordStore struct {
	Session *gocql.Session
}

func NewClinicalRecordStore(session *gocql.Session) *ClinicalRecordStore {
	return &ClinicalRecordStore{Session: session}
}

// StoredRecord é um registro clínico lido do Cassandra.
type StoredRecord struct {
	ID             string
	PatientID      string
	OriginClinicID string
	FHIRResourceID string
	EffectiveAt    time.Time
	FHIRJSON       string
}

// Upsert grava (ou atualiza) o prontuário de uma clínica para um paciente. Devolve o id do registro.
func (s *ClinicalRecordStore) Upsert(ctx context.Context, patientID, clinicID, fhirID string, effective time.Time, title, conclusion, fhirJSON string) (string, error) {
	var recordID, oldPatient string
	var oldEffective time.Time
	err := s.Session.Query(
		`SELECT record_id, patient_id, effective_at FROM clinical_record_by_origin WHERE origin_clinic = ? AND fhir_id = ?`,
		clinicID, fhirID,
	).WithContext(ctx).Scan(&recordID, &oldPatient, &oldEffective)

	switch {
	case errors.Is(err, gocql.ErrNotFound):
		recordID = gocql.TimeUUID().String()
	case err != nil:
		return "", fmt.Errorf("falha ao localizar prontuário: %w", err)
	default:
		// Reenvio: remove a linha antiga do paciente antes de gravar a nova (a data pode ter mudado).
		if err := s.Session.Query(
			`DELETE FROM clinical_record_by_patient WHERE patient_id = ? AND effective_at = ? AND record_id = ?`,
			oldPatient, oldEffective, recordID,
		).WithContext(ctx).Exec(); err != nil {
			return "", fmt.Errorf("falha ao atualizar prontuário (remoção): %w", err)
		}
	}

	if err := s.Session.Query(
		`INSERT INTO clinical_record_by_patient (patient_id, effective_at, record_id, origin_clinic, fhir_id, title, conclusion, fhir_json) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		patientID, effective, recordID, clinicID, fhirID, title, conclusion, fhirJSON,
	).WithContext(ctx).Exec(); err != nil {
		return "", fmt.Errorf("falha ao gravar prontuário: %w", err)
	}
	if err := s.Session.Query(
		`INSERT INTO clinical_record_by_origin (origin_clinic, fhir_id, record_id, patient_id, effective_at) VALUES (?, ?, ?, ?, ?)`,
		clinicID, fhirID, recordID, patientID, effective,
	).WithContext(ctx).Exec(); err != nil {
		return "", fmt.Errorf("falha ao indexar prontuário: %w", err)
	}
	return recordID, nil
}

// ListByPatient devolve os prontuários do paciente, do mais recente para o mais antigo (até 500).
func (s *ClinicalRecordStore) ListByPatient(ctx context.Context, patientID string) ([]StoredRecord, error) {
	iter := s.Session.Query(
		`SELECT record_id, origin_clinic, fhir_id, effective_at, fhir_json FROM clinical_record_by_patient WHERE patient_id = ? LIMIT 500`,
		patientID,
	).WithContext(ctx).Iter()
	var out []StoredRecord
	var recordID, origin, fhirID, fhirJSON string
	var effective time.Time
	for iter.Scan(&recordID, &origin, &fhirID, &effective, &fhirJSON) {
		out = append(out, StoredRecord{
			ID:             recordID,
			PatientID:      patientID,
			OriginClinicID: origin,
			FHIRResourceID: fhirID,
			EffectiveAt:    effective,
			FHIRJSON:       fhirJSON,
		})
	}
	if err := iter.Close(); err != nil {
		return nil, fmt.Errorf("falha ao ler prontuários: %w", err)
	}
	return out, nil
}

// validFHIRJSON garante que o conteúdo guardado é JSON válido (evita gravar lixo no histórico).
func validFHIRJSON(raw []byte) bool {
	var v map[string]interface{}
	return json.Unmarshal(raw, &v) == nil
}

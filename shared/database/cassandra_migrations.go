package database

import (
	"log"

	"github.com/gocql/gocql"
)

func RunCassandraMigrations(session *gocql.Session) error {
	log.Println("⚙️ Iniciando estruturação das tabelas do Cassandra...")

	// Criação da tabela user_devices
	createDevicesTable := `CREATE TABLE IF NOT EXISTS user_devices (
		user_id uuid,
		created_at bigint,
		device_name text,
		public_key text,
		PRIMARY KEY (user_id, created_at)
	) WITH CLUSTERING ORDER BY (created_at DESC);`
	
	if err := session.Query(createDevicesTable).Exec(); err != nil {
		log.Printf("⚠️ Aviso ao criar tabela user_devices: %v", err)
		return err
	}

	// Criação da tabela register_logs
	createLogsTable := `CREATE TABLE IF NOT EXISTS register_logs (
		log_id timeuuid,
		event_hour timestamp,
		reference_date timestamp,
		origin_service text,
		action_type text,
		description text,
		origin_ip text,
		result_status text,
		user_id uuid,
		PRIMARY KEY (log_id)
	);`
	
	if err := session.Query(createLogsTable).Exec(); err != nil {
		log.Printf("⚠️ Aviso ao criar tabela register_logs: %v", err)
		return err
	}

	// Prontuários FHIR das clínicas: por paciente (histórico) e por clínica de origem + id FHIR (reenvio).
	recordByPatient := `CREATE TABLE IF NOT EXISTS clinical_record_by_patient (
		patient_id text,
		effective_at timestamp,
		record_id text,
		origin_clinic text,
		fhir_id text,
		title text,
		conclusion text,
		fhir_json text,
		PRIMARY KEY (patient_id, effective_at, record_id)
	) WITH CLUSTERING ORDER BY (effective_at DESC, record_id ASC);`
	if err := session.Query(recordByPatient).Exec(); err != nil {
		log.Printf("⚠️ Aviso ao criar tabela clinical_record_by_patient: %v", err)
		return err
	}

	recordByOrigin := `CREATE TABLE IF NOT EXISTS clinical_record_by_origin (
		origin_clinic text,
		fhir_id text,
		record_id text,
		patient_id text,
		effective_at timestamp,
		PRIMARY KEY (origin_clinic, fhir_id)
	);`
	if err := session.Query(recordByOrigin).Exec(); err != nil {
		log.Printf("⚠️ Aviso ao criar tabela clinical_record_by_origin: %v", err)
		return err
	}

	// Auditoria de acesso (LGPD): por ator e por paciente.
	auditByActor := `CREATE TABLE IF NOT EXISTS access_audit_by_actor (
		actor text,
		event_time timeuuid,
		action text,
		patient_id text,
		result text,
		PRIMARY KEY (actor, event_time)
	) WITH CLUSTERING ORDER BY (event_time DESC);`
	if err := session.Query(auditByActor).Exec(); err != nil {
		log.Printf("⚠️ Aviso ao criar tabela access_audit_by_actor: %v", err)
		return err
	}

	auditByPatient := `CREATE TABLE IF NOT EXISTS access_audit_by_patient (
		patient_id text,
		event_time timeuuid,
		actor text,
		action text,
		result text,
		PRIMARY KEY (patient_id, event_time)
	) WITH CLUSTERING ORDER BY (event_time DESC);`
	if err := session.Query(auditByPatient).Exec(); err != nil {
		log.Printf("⚠️ Aviso ao criar tabela access_audit_by_patient: %v", err)
		return err
	}

	log.Println("✅ Estruturas do Cassandra verificadas/criadas com sucesso!")
	return nil
}

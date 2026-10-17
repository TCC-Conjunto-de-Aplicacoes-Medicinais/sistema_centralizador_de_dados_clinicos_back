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

	// Criação da tabela patient_clinical_records (Pluralidade de Clínicas & FHIR Time-Series)
	createClinicalRecordsTable := `CREATE TABLE IF NOT EXISTS patient_clinical_records (
		patient_id text,
		exam_timestamp timestamp,
		record_id text,
		clinic_id text,
		clinic_name text,
		resource_type text,
		title text,
		conclusion text,
		fhir_payload text,
		PRIMARY KEY (patient_id, exam_timestamp, record_id)
	) WITH CLUSTERING ORDER BY (exam_timestamp DESC);`

	if err := session.Query(createClinicalRecordsTable).Exec(); err != nil {
		log.Printf("⚠️ Aviso ao criar tabela patient_clinical_records no Cassandra: %v", err)
		return err
	}

	log.Println("✅ Estruturas do Cassandra (user_devices, register_logs, patient_clinical_records) verificadas/criadas com sucesso!")
	return nil
}

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

	log.Println("✅ Estruturas do Cassandra verificadas/criadas com sucesso!")
	return nil
}

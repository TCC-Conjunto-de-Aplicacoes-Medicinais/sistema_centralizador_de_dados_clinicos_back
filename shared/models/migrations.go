package models

import (
	"log"

	"gorm.io/gorm"
)

// RunMigrations executa o AutoMigrate (criação/atualização das tabelas) para todas as structs GORM cadastradas.
func RunMigrations(db *gorm.DB) error {
	log.Println("⚙️ Iniciando sincronização e estruturação do banco de dados (MariaDB)...")

	// Ponto central de registro: sempre que você criar uma model nova (User, Appointments, etc), adicione ela na lista abaixo!
	err := db.AutoMigrate(
		&Patients{}, 
		// &Doctors{},
		// &Appointments{},
		// etc...
	)

	if err != nil {
		return err
	}

	log.Println("✅ Todas as tabelas SQL da clínica foram alinhadas e atualizadas com sucesso!")
	return nil
}

package database

import (
	"log"

	"gorm.io/gorm"
)

func RunMigrations(db *gorm.DB) error {
	log.Println("⚙️ Iniciando sincronização e estruturação do banco de dados (MariaDB)...")

	err := db.AutoMigrate(
		&Patient{},
		&Doctor{},
		&Clinic{},
		&ClinicDoctor{},
		&Exam{},
		&DoctorPermission{},
		&PatientEmail{},
		&PatientPhone{},
		&PatientAddress{},
		&ClinicalUser{},
		&PatientToken{},
		&AccessAuditLog{},
	)

	if err != nil {
		return err
	}

	log.Println("✅ Todas as tabelas SQL da clínica foram alinhadas e atualizadas com sucesso!")
	return nil
}

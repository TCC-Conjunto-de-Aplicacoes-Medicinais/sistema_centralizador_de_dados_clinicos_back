package database

import (
	"log"

	"gorm.io/gorm"
)

func RunMigrations(db *gorm.DB) error {
	log.Println("⚙️ Iniciando sincronização e estruturação do banco de dados (MariaDB)...")

	err := db.AutoMigrate(
		&Patients{},
		&Doctors{},
		&Clinics{},
		&ClinicDoctor{},
		&Exams{},
		&DoctorPermission{},
		// &Appointments{},
		// etc...
	)

	if err != nil {
		return err
	}

	log.Println("✅ Todas as tabelas SQL da clínica foram alinhadas e atualizadas com sucesso!")
	return nil
}

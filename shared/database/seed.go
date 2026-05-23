package database

import (
	"log"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func SeedDemoData(db *gorm.DB) error {
	log.Println("🌱 Verificando necessidade de povoamento inicial de dados (seeding)...")

	// 1. Seed Clinics
	var countClinics int64
	if err := db.Model(&Clinic{}).Count(&countClinics).Error; err != nil {
		return err
	}

	if countClinics == 0 {
		log.Println("🌱 Inserindo clínicas fictícias de teste...")
		clinics := []Clinic{
			{
				ID:              "CLI-1001",
				CNPJ:            "11111111000111",
				Email:           "contato@vida.com.br",
				ResponsibleName: "Dr. Roberto Silva",
				ClinicName:      "Clínica Vida Saudável",
				Location:        "São Paulo, SP",
				Specialty:       "Geral / Cardiologia",
				Phone:           "(11) 3333-4444",
				Verify:          true,
				Type:            "internal",
				ApiURL:          "http://api.vida.com.br/webhook",
			},
			{
				ID:              "CLI-2002",
				CNPJ:            "22222222000122",
				Email:           "contato@saolucas.com.br",
				ResponsibleName: "Dra. Fernanda Souza",
				ClinicName:      "Hospital Metropolitano São Lucas",
				Location:        "Rio de Janeiro, RJ",
				Specialty:       "Pediatria / Geral",
				Phone:           "(21) 4444-5555",
				Verify:          true,
				Type:            "internal",
				ApiURL:          "http://api.saolucas.com.br/webhook",
			},
			{
				ID:              "CLI-3003",
				CNPJ:            "33333333000133",
				Email:           "contato@cardiocentro.com.br",
				ResponsibleName: "Dra. Ana Paula",
				ClinicName:      "CardioCentro Integrado",
				Location:        "Belo Horizonte, MG",
				Specialty:       "Cardiologia",
				Phone:           "(31) 5555-6666",
				Verify:          true,
				Type:            "partner",
				ApiURL:          "http://api.cardiocentro.com.br/webhook",
			},
			{
				ID:              "CLI-4004",
				CNPJ:            "44444444000144",
				Email:           "contato@santacecilia.com.br",
				ResponsibleName: "Dr. Thiago Costa",
				ClinicName:      "Laboratório Santa Cecília",
				Location:        "Campinas, SP",
				Specialty:       "Análises Clínicas",
				Phone:           "(19) 6666-7777",
				Verify:          true,
				Type:            "partner",
				ApiURL:          "http://api.santacecilia.com.br/webhook",
			},
		}

		for _, clinic := range clinics {
			if err := db.Create(&clinic).Error; err != nil {
				return err
			}
		}

		// Seed Clinical Users
		log.Println("🌱 Inserindo usuários de clínica de teste...")
		hash, err := bcrypt.GenerateFromPassword([]byte("senha123"), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		passwordHash := string(hash)

		clinicalUsers := []ClinicalUser{
			{
				ClinicID:     "CLI-1001",
				Email:        "roberto.silva@vida.com.br",
				PasswordHash: passwordHash,
				FullName:     "Dr. Roberto Silva",
				Role:         "Médico Cardiologista",
				Active:       true,
			},
			{
				ClinicID:     "CLI-2002",
				Email:        "fernanda.souza@saolucas.com.br",
				PasswordHash: passwordHash,
				FullName:     "Dra. Fernanda Souza",
				Role:         "Médica Pediatra",
				Active:       true,
			},
			{
				ClinicID:     "CLI-3003",
				Email:        "ana.paula@cardiocentro.com.br",
				PasswordHash: passwordHash,
				FullName:     "Dra. Ana Paula",
				Role:         "Clínica Geral (Parceira)",
				Active:       true,
			},
			{
				ClinicID:     "CLI-4004",
				Email:        "thiago.costa@santacecilia.com.br",
				PasswordHash: passwordHash,
				FullName:     "Dr. Thiago Costa",
				Role:         "Médico Patologista (Parceiro)",
				Active:       true,
			},
		}

		for _, user := range clinicalUsers {
			if err := db.Create(&user).Error; err != nil {
				return err
			}
		}
	}

	// 2. Seed Patients
	var countPatients int64
	if err := db.Model(&Patient{}).Count(&countPatients).Error; err != nil {
		return err
	}

	if countPatients == 0 {
		log.Println("🌱 Inserindo pacientes de teste...")

		birth1, _ := time.Parse("2006-01-02", "1988-04-12")
		birth2, _ := time.Parse("2006-01-02", "1975-08-25")
		birth3, _ := time.Parse("2006-01-02", "1995-11-02")
		birth4, _ := time.Parse("2006-01-02", "2002-12-05")
		birth5, _ := time.Parse("2006-01-02", "1960-03-30")

		patients := []Patient{
			{
				Id:               "pat-1",
				Name:             "Maria Oliveira Souza",
				CPF:              "12345678909",
				BirthDate:        birth1,
				Gender:           "feminino",
				EmergencyContact: "(11) 98765-4321",
				Verify:           true,
				Allergies:        `["Dipirona Monoidratada", "Poeira/Ácaros", "PenicilinaG"]`,
				Medications:      `["Losartana Potássica 50mg (1 comprimido a cada 12 horas)", "Metformina 850mg (1 comprimido no almoço e janta)", "Vitamina D 2000 UI (1 gota ao dia)"]`,
			},
			{
				Id:               "pat-2",
				Name:             "João Silva Santos",
				CPF:              "98765432100",
				BirthDate:        birth2,
				Gender:           "masculino",
				EmergencyContact: "(21) 99888-7766",
				Verify:           true,
				Allergies:        `["Ácido Acetilsalicílico (AAS)", "Lactose", "Iodo (Contraste)"]`,
				Medications:      `["Atorvastatina 20mg (1 comprimido à noite)", "Omeprazol 20mg (1 comprimido pela manhã em jejum)"]`,
			},
			{
				Id:               "pat-3",
				Name:             "Carlos Eduardo Costa",
				CPF:              "45678912300",
				BirthDate:        birth3,
				Gender:           "masculino",
				EmergencyContact: "(31) 97555-4433",
				Verify:           true,
				Allergies:        `["Picada de Abelha / Himenópteros", "Látex natural"]`,
				Medications:      `[]`,
			},
			{
				Id:               "pat-4",
				Name:             "Ana Julia Ribeiro",
				CPF:              "55566677708",
				BirthDate:        birth4,
				Gender:           "feminino",
				EmergencyContact: "(11) 96543-2109",
				Verify:           true,
				Allergies:        `["Sulfa / Sulfonamidas"]`,
				Medications:      `["Anticoncepcional Oral (1x ao dia)"]`,
			},
			{
				Id:               "pat-5",
				Name:             "Marcos Paulo Souza",
				CPF:              "22233344405",
				BirthDate:        birth5,
				Gender:           "masculino",
				EmergencyContact: "(19) 99111-2233",
				Verify:           true,
				Allergies:        `["Dipirona Monoidratada", "Penicilina"]`,
				Medications:      `["Atenolol 25mg (1x ao dia)", "Losartana Potássica 50mg (1x ao dia)", "Sinvastatina 20mg (1x à noite)"]`,
			},
		}

		for _, patient := range patients {
			if err := db.Create(&patient).Error; err != nil {
				return err
			}

			// Seed patient details (Emails and Phones)
			email := PatientEmail{
				Id:        "email-" + patient.Id,
				PatientID: patient.Id,
				Email:     patient.Id + "@gmail.com",
				Principal: true,
			}
			db.Create(&email)

			phone := PatientPhone{
				Id:        "phone-" + patient.Id,
				PatientID: patient.Id,
				Phone:     patient.EmergencyContact,
				Principal: true,
			}
			db.Create(&phone)
		}

		// Seed Patient Tokens OTP (valid for 1 year to guarantee they won't expire during tests)
		log.Println("🌱 Inserindo tokens OTP de teste...")
		expiry := time.Now().Add(8760 * time.Hour) // 1 ano
		tokens := []PatientToken{
			{PatientID: "pat-1", TokenCode: "123456", ExpiresAt: expiry},
			{PatientID: "pat-2", TokenCode: "654321", ExpiresAt: expiry},
			{PatientID: "pat-3", TokenCode: "987654", ExpiresAt: expiry},
			{PatientID: "pat-4", TokenCode: "246810", ExpiresAt: expiry},
			{PatientID: "pat-5", TokenCode: "135790", ExpiresAt: expiry},
		}

		for _, token := range tokens {
			db.Create(&token)
		}

		// Seed Exams
		log.Println("🌱 Inserindo exames de teste...")
		exams := []Exam{
			{
				Id:          "ex-1",
				PatientId:   "pat-1",
				ClinicId:    "CLI-4004",
				LinkBucket:  "laudo_hemograma_maria.pdf",
				IdCassandra: "00000000-0000-0000-0000-000000000001",
				FlagActive:  true,
				Title:       "Hemograma Completo",
				Provider:    "Laboratório Santa Cecília",
				Result:      "Anemia leve identificada (Hemoglobina: 11.2 g/dL), demais parâmetros dentro do padrão referencial.",
			},
			{
				Id:          "ex-2",
				PatientId:   "pat-1",
				ClinicId:    "CLI-3003",
				LinkBucket:  "ecg_maria.pdf",
				IdCassandra: "00000000-0000-0000-0000-000000000002",
				FlagActive:  true,
				Title:       "Eletrocardiograma (ECG)",
				Provider:    "CardioCentro Integrado",
				Result:      "Ritmo sinusal regular, frequência cardíaca média de 72 bpm, sem evidência de alterações de repolarização.",
			},
			{
				Id:          "ex-3",
				PatientId:   "pat-2",
				ClinicId:    "CLI-4004",
				LinkBucket:  "glicemia_joao.pdf",
				IdCassandra: "00000000-0000-0000-0000-000000000003",
				FlagActive:  true,
				Title:       "Glicemia de Jejum",
				Provider:    "Laboratório Santa Cecília",
				Result:      "Glicose sérica: 96 mg/dL. Valores de referência: 70 a 99 mg/dL (Desejável).",
			},
			{
				Id:          "ex-4",
				PatientId:   "pat-4",
				ClinicId:    "CLI-4004",
				LinkBucket:  "beta_hcg_ana.pdf",
				IdCassandra: "00000000-0000-0000-0000-000000000004",
				FlagActive:  true,
				Title:       "Beta HCG Quantitativo",
				Provider:    "Laboratório Santa Cecília",
				Result:      "Resultado: Negativo (< 2.0 mUI/mL). Ausência de gravidez no momento.",
			},
			{
				Id:          "ex-5",
				PatientId:   "pat-5",
				ClinicId:    "CLI-4004",
				LinkBucket:  "lipideo_marcos.pdf",
				IdCassandra: "00000000-0000-0000-0000-000000000005",
				FlagActive:  true,
				Title:       "Perfil Lipídico",
				Provider:    "Laboratório Santa Cecília",
				Result:      "Colesterol Total: 185 mg/dL, HDL: 45 mg/dL, LDL: 110 mg/dL, Triglicerídeos: 150 mg/dL. Risco cardiovascular baixo.",
			},
		}

		for _, exam := range exams {
			db.Create(&exam)
		}
	}

	log.Println("✅ Carga inicial de dados finalizada com sucesso!")
	return nil
}

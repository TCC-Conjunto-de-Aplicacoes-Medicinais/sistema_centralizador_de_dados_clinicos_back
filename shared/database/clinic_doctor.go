package database

type ClinicDoctor struct {
	ClinicID string `gorm:"type:char(36);primaryKey" json:"clinic_id"`
	DoctorID string `gorm:"type:char(36);primaryKey" json:"doctor_id"`

	Doctor Doctors `gorm:"foreignKey:DoctorID;references:Id" json:"doctor"`
	Clinic Clinics `gorm:"foreignKey:ClinicID;references:ID" json:"clinic"`
}

func (ClinicDoctor) TableName() string {
	return "clinic_doctors"
}

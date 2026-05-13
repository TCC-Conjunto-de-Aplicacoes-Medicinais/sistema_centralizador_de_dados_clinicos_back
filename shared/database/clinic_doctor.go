package database

type ClinicDoctor struct {
	ClinicID string `gorm:"type:char(36);primaryKey" json:"clinic_id"`
	DoctorID string `gorm:"type:char(36);primaryKey" json:"doctor_id"`

	Doctor Doctor `gorm:"foreignKey:DoctorID;references:Id" json:"doctor"`
	Clinic Clinic `gorm:"foreignKey:ClinicID;references:ID" json:"clinic"`
}

func (ClinicDoctor) TableName() string {
	return "clinic_doctor"
}

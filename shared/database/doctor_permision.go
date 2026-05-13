package database

import "gorm.io/gorm"

type DoctorPermission struct {
	gorm.Model
	Id            string `gorm:"column:id;type:char(36);primaryKey" json:"id"`
	DoctorID      string `gorm:"column:doctor_id;type:char(36);not null" json:"doctor_id"`
	ExamID        string `gorm:"column:exam_id;type:char(36);not null" json:"exam_id"`
	BreakTheGlass string `gorm:"column:break_the_glass;type:varchar(100);" json:"break_the_glass"`

	Doctor Doctor `gorm:"foreignKey:DoctorID;references:Id" json:"doctor"`
	Exam   Exam   `gorm:"foreignKey:ExamID;references:Id" json:"exam"`
}

func (DoctorPermission) TableName() string {
	return "doctor_permission"
}

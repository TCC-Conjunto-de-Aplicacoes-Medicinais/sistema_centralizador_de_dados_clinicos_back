package database

import "gorm.io/gorm"

type PatientEmail struct {
	gorm.Model
	Id        string `gorm:"column:id;type:char(36);primaryKey" json:"id"`
	PatientID string `gorm:"column:patient_id;type:char(36);not null" json:"patient_id"`
	Email     string `gorm:"column:email;type:varchar(150);not null" json:"email"`
	Principal bool   `gorm:"column:principal;type:boolean;not null;default:false" json:"principal"`
}

func (PatientEmail) TableName() string {
	return "patient_emails"
}

type PatientPhone struct {
	gorm.Model
	Id        string `gorm:"column:id;type:char(36);primaryKey" json:"id"`
	PatientID string `gorm:"column:patient_id;type:char(36);not null" json:"patient_id"`
	Phone     string `gorm:"column:phone;type:varchar(20);not null" json:"phone"`
	Principal bool   `gorm:"column:principal;type:boolean;not null;default:false" json:"principal"`
}

func (PatientPhone) TableName() string {
	return "patient_phones"
}

type PatientAddress struct {
	gorm.Model
	Id        string `gorm:"column:id;type:char(36);primaryKey" json:"id"`
	PatientID string `gorm:"column:patient_id;type:char(36);not null" json:"patient_id"`
	Address   string `gorm:"column:address;type:varchar(255);not null" json:"address"`
	Principal bool   `gorm:"column:principal;type:boolean;not null;default:false" json:"principal"`
}

func (PatientAddress) TableName() string {
	return "patient_addresses"
}

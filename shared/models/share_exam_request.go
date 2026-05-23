package models

type ShareExamRequest struct {
	ExamID     string `json:"exam_id" binding:"required"`
	DoctorName string `json:"doctor_name" binding:"required"`
}

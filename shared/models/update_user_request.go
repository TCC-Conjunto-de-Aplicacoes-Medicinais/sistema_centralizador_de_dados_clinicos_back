package models

type UpdateUserRequest struct {
	Name             string           `json:"name,omitempty"`
	Emails           []EmailRequest   `json:"emails,omitempty"`
	Phones           []PhoneRequest   `json:"phones,omitempty"`
	Addresses        []AddressRequest `json:"addresses,omitempty"`
	EmergencyContact string           `json:"emergency_contact,omitempty"`
}

type EmailRequest struct {
	Email     string `json:"email" binding:"required,email"`
	Principal bool   `json:"principal"`
}

type PhoneRequest struct {
	Phone     string `json:"phone" binding:"required"`
	Principal bool   `json:"principal"`
}

type AddressRequest struct {
	Address   string `json:"address" binding:"required"`
	Principal bool   `json:"principal"`
}

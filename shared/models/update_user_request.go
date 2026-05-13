package models

type UpdateUserRequest struct {
	Name             string `json:"name,omitempty"`
	Phone            string `json:"phone,omitempty"`
	Address          string `json:"address,omitempty"`
	EmergencyContact string `json:"emergency_contact,omitempty"`
}

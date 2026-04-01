package models

type SignupRequest struct {
	Name       string `json:"name"        binding:"required"`
	Email      string `json:"email"       binding:"required,email"`
	Password   string `json:"password"    binding:"required,min=8"`
	CPF        string `json:"cpf"         binding:"required,len=11"`
	Device     Device `json:"device"      binding:"required"`
}

type Device struct {
	PublicKey  string `json:"public_key"  binding:"required"`
	DeviceName string `json:"device_name" binding:"required"`
}
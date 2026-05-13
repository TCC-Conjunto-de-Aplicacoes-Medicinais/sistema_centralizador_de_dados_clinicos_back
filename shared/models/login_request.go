package models

type LoginRequest struct {
	CPF      string `json:"cpf"      binding:"required,len=11"`
	Password string `json:"password" binding:"required,min=8"`
}

type LoginResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
}

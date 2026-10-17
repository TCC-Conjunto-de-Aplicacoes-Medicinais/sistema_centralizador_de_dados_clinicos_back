package models

// AIAnalysisRequest contém o texto do paciente solicitando análise de exames.
type AIAnalysisRequest struct {
	Query string `json:"query" binding:"required"`
}

// AIAnalysisResponse contém a análise gerada pela IA e o disclaimer obrigatório.
type AIAnalysisResponse struct {
	Analysis   string `json:"analysis"`
	Disclaimer string `json:"disclaimer"`
}

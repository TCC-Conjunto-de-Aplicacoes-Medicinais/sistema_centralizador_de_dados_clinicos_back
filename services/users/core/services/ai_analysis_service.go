package services

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/logger"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/models"
	"gorm.io/gorm"
)

const (
	// disclaimerPT é o aviso obrigatório anexado ao final de toda resposta da IA.
	disclaimerPT = "⚠️ AVISO IMPORTANTE: Este diagnóstico foi gerado por inteligência artificial e NÃO deve ser " +
		"considerado como verdade absoluta. A IA é apenas uma ferramenta de segunda opinião. " +
		"Recomendamos fortemente que você consulte um médico de confiança para uma avaliação profissional e diagnóstico preciso."

	// systemPrompt define o comportamento do agente médico.
	systemPrompt = `Você é um assistente médico de segunda opinião baseado em inteligência artificial. 
Seu papel é ajudar pacientes a entender melhor seus exames e sintomas, oferecendo uma análise informativa.

Regras:
1. Analise o texto do paciente com cuidado e empatia.
2. Se o paciente descrever resultados de exames, interprete os valores e explique o que podem significar.
3. Se o paciente descrever sintomas, sugira possíveis condições relacionadas.
4. Sempre estruture sua resposta de forma clara, usando seções quando apropriado.
5. Use linguagem acessível, evitando jargão médico excessivo, mas mantenha a precisão.
6. Nunca prescreva medicamentos.
7. Nunca afirme diagnósticos com certeza absoluta — use termos como "pode indicar", "sugere", "é possível que".
8. Responda sempre em português do Brasil.
9. NÃO inclua disclaimers ou avisos na sua resposta — isso será adicionado automaticamente pelo sistema.`

	geminiAPIURL = "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash:generateContent"
)

// AIAnalysisService é o serviço responsável por processar análises médicas via IA.
type AIAnalysisService struct {
	DB           *gorm.DB
	GeminiAPIKey string
	HTTPClient   *http.Client
	Logger       *logger.Logger
}

// NewAIAnalysisService cria uma nova instância do serviço de análise por IA.
func NewAIAnalysisService(db *gorm.DB, geminiAPIKey string, l *logger.Logger) *AIAnalysisService {
	return &AIAnalysisService{
		DB:           db,
		GeminiAPIKey: geminiAPIKey,
		HTTPClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		Logger: l,
	}
}

// -------------------------------------------------------------------
// Estruturas para a API Gemini (Request/Response)
// -------------------------------------------------------------------

type geminiRequest struct {
	SystemInstruction *geminiContent  `json:"systemInstruction,omitempty"`
	Contents          []geminiContent `json:"contents"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	Error *struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
	} `json:"error,omitempty"`
}

// -------------------------------------------------------------------
// Método principal de análise
// -------------------------------------------------------------------

// Analyze processa a consulta do paciente e retorna a análise da IA.
func (s *AIAnalysisService) Analyze(userID string, req models.AIAnalysisRequest) (*models.AIAnalysisResponse, error) {
	if s.GeminiAPIKey == "" {
		s.log(logger.LogEntry{
			OriginService: "ai_agent",
			ActionType:    "ai_analyze",
			Description:   "GEMINI_API_KEY não configurada — serviço de IA indisponível",
			ResultStatus:  "warning",
			UserID:        userID,
		})
		return nil, errors.New("chave da API Gemini não configurada (GEMINI_API_KEY)")
	}

	// Log: início da análise
	queryPreview := req.Query
	if len(queryPreview) > 100 {
		queryPreview = queryPreview[:100] + "..."
	}
	s.log(logger.LogEntry{
		OriginService: "ai_agent",
		ActionType:    "ai_analyze",
		Description:   fmt.Sprintf("iniciando análise de IA para usuário %s — consulta: %s", userID, queryPreview),
		ResultStatus:  "success",
		UserID:        userID,
	})

	// ---------------------------------------------------------------
	// TODO: Buscar dados de exames do paciente no banco de dados.
	// Quando implementado, os dados dos exames serão anexados ao prompt
	// para fornecer contexto adicional à IA.
	//
	// Exemplo futuro:
	//   patientExams, err := s.fetchPatientExams(userID)
	//   if err != nil { ... }
	//   enrichedQuery := fmt.Sprintf("Dados do paciente:\n%s\n\nConsulta:\n%s", patientExams, req.Query)
	// ---------------------------------------------------------------

	// Por enquanto, usamos apenas o texto enviado pelo paciente
	userQuery := req.Query

	// Chama a API Gemini
	analysisText, err := s.callGeminiAPI(userID, userQuery)
	if err != nil {
		return nil, fmt.Errorf("erro ao consultar IA: %w", err)
	}

	// Warning: resposta muito curta da IA
	if len(strings.TrimSpace(analysisText)) < 20 {
		s.log(logger.LogEntry{
			OriginService: "ai_agent",
			ActionType:    "ai_analyze",
			Description:   fmt.Sprintf("resposta da IA muito curta para usuário %s (%d caracteres)", userID, len(analysisText)),
			ResultStatus:  "warning",
			UserID:        userID,
		})
	}

	// Log: análise concluída com sucesso
	s.log(logger.LogEntry{
		OriginService: "ai_agent",
		ActionType:    "ai_analyze",
		Description:   fmt.Sprintf("análise de IA concluída com sucesso para usuário %s — resposta com %d caracteres", userID, len(analysisText)),
		ResultStatus:  "success",
		UserID:        userID,
	})

	return &models.AIAnalysisResponse{
		Analysis:   analysisText,
		Disclaimer: disclaimerPT,
	}, nil
}

// callGeminiAPI envia o prompt para a API Gemini e retorna o texto gerado.
func (s *AIAnalysisService) callGeminiAPI(userID string, userQuery string) (string, error) {
	reqBody := geminiRequest{
		SystemInstruction: &geminiContent{
			Parts: []geminiPart{{Text: systemPrompt}},
		},
		Contents: []geminiContent{
			{
				Parts: []geminiPart{{Text: userQuery}},
			},
		},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		s.log(logger.LogEntry{
			OriginService: "ai_agent",
			ActionType:    "ai_gemini_call",
			Description:   "erro ao serializar request para API Gemini: " + err.Error(),
			ResultStatus:  "error",
			UserID:        userID,
		})
		return "", fmt.Errorf("erro ao serializar request: %w", err)
	}

	url := fmt.Sprintf("%s?key=%s", geminiAPIURL, s.GeminiAPIKey)

	httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		s.log(logger.LogEntry{
			OriginService: "ai_agent",
			ActionType:    "ai_gemini_call",
			Description:   "erro ao criar request HTTP para API Gemini: " + err.Error(),
			ResultStatus:  "error",
			UserID:        userID,
		})
		return "", fmt.Errorf("erro ao criar request HTTP: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	s.log(logger.LogEntry{
		OriginService: "ai_agent",
		ActionType:    "ai_gemini_call",
		Description:   fmt.Sprintf("enviando requisição para API Gemini (usuário %s)", userID),
		ResultStatus:  "success",
		UserID:        userID,
	})

	resp, err := s.HTTPClient.Do(httpReq)
	if err != nil {
		s.log(logger.LogEntry{
			OriginService: "ai_agent",
			ActionType:    "ai_gemini_call",
			Description:   "falha na conexão com API Gemini: " + err.Error(),
			ResultStatus:  "error",
			UserID:        userID,
		})
		return "", fmt.Errorf("erro na chamada à API Gemini: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		s.log(logger.LogEntry{
			OriginService: "ai_agent",
			ActionType:    "ai_gemini_call",
			Description:   "erro ao ler corpo da resposta da API Gemini: " + err.Error(),
			ResultStatus:  "error",
			UserID:        userID,
		})
		return "", fmt.Errorf("erro ao ler resposta da API: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		s.log(logger.LogEntry{
			OriginService: "ai_agent",
			ActionType:    "ai_gemini_call",
			Description:   fmt.Sprintf("API Gemini retornou status HTTP %d para usuário %s", resp.StatusCode, userID),
			ResultStatus:  "error",
			UserID:        userID,
		})
		return "", fmt.Errorf("API Gemini retornou status %d: %s", resp.StatusCode, string(body))
	}

	var geminiResp geminiResponse
	if err := json.Unmarshal(body, &geminiResp); err != nil {
		s.log(logger.LogEntry{
			OriginService: "ai_agent",
			ActionType:    "ai_gemini_call",
			Description:   "erro ao decodificar resposta JSON da API Gemini: " + err.Error(),
			ResultStatus:  "error",
			UserID:        userID,
		})
		return "", fmt.Errorf("erro ao decodificar resposta da API: %w", err)
	}

	if geminiResp.Error != nil {
		s.log(logger.LogEntry{
			OriginService: "ai_agent",
			ActionType:    "ai_gemini_call",
			Description:   "erro retornado pela API Gemini: " + geminiResp.Error.Message,
			ResultStatus:  "error",
			UserID:        userID,
		})
		return "", fmt.Errorf("erro da API Gemini: %s", geminiResp.Error.Message)
	}

	if len(geminiResp.Candidates) == 0 ||
		len(geminiResp.Candidates[0].Content.Parts) == 0 {
		s.log(logger.LogEntry{
			OriginService: "ai_agent",
			ActionType:    "ai_gemini_call",
			Description:   fmt.Sprintf("API Gemini retornou resposta vazia para usuário %s", userID),
			ResultStatus:  "warning",
			UserID:        userID,
		})
		return "", errors.New("a API Gemini não retornou nenhuma resposta")
	}

	s.log(logger.LogEntry{
		OriginService: "ai_agent",
		ActionType:    "ai_gemini_call",
		Description:   fmt.Sprintf("resposta recebida da API Gemini com sucesso para usuário %s", userID),
		ResultStatus:  "success",
		UserID:        userID,
	})

	return geminiResp.Candidates[0].Content.Parts[0].Text, nil
}

// log é um helper que registra entradas no Cassandra de forma segura (nil-safe).
func (s *AIAnalysisService) log(entry logger.LogEntry) {
	if s.Logger != nil {
		s.Logger.Log(entry)
	}
}

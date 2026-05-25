package services

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/logger"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/models"
	"google.golang.org/genai"
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
// Analyze processa a consulta do paciente e retorna a análise da IA.
func (s *AIAnalysisService) Analyze(ctx context.Context, userID string, req models.AIAnalysisRequest) (*models.AIAnalysisResponse, error) {
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

	// Chama a API Gemini usando o SDK
	analysisText, err := s.callGeminiAPI(ctx, userID, userQuery)
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

// callGeminiAPI envia o prompt para a API Gemini via SDK oficial e retorna o texto gerado.
func (s *AIAnalysisService) callGeminiAPI(ctx context.Context, userID string, userQuery string) (string, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:     s.GeminiAPIKey,
		HTTPClient: s.HTTPClient,
	})
	if err != nil {
		s.log(logger.LogEntry{
			OriginService: "ai_agent",
			ActionType:    "ai_gemini_call",
			Description:   "erro ao inicializar cliente GenAI: " + err.Error(),
			ResultStatus:  "error",
			UserID:        userID,
		})
		return "", fmt.Errorf("erro ao inicializar cliente GenAI: %w", err)
	}

	s.log(logger.LogEntry{
		OriginService: "ai_agent",
		ActionType:    "ai_gemini_call",
		Description:   fmt.Sprintf("enviando requisição para API Gemini via SDK (usuário %s)", userID),
		ResultStatus:  "success",
		UserID:        userID,
	})

	config := &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{
				{Text: systemPrompt},
			},
		},
	}

	resp, err := client.Models.GenerateContent(
		ctx,
		"gemini-2.5-flash",
		genai.Text(userQuery),
		config,
	)
	if err != nil {
		s.log(logger.LogEntry{
			OriginService: "ai_agent",
			ActionType:    "ai_gemini_call",
			Description:   "falha na chamada à API Gemini via SDK: " + err.Error(),
			ResultStatus:  "error",
			UserID:        userID,
		})
		return "", fmt.Errorf("erro na chamada ao modelo Gemini: %w", err)
	}

	analysisText := resp.Text()
	if analysisText == "" {
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
		Description:   fmt.Sprintf("resposta recebida do SDK Gemini com sucesso para usuário %s", userID),
		ResultStatus:  "success",
		UserID:        userID,
	})

	return analysisText, nil
}

// log é um helper que registra entradas no Cassandra de forma segura (nil-safe).
func (s *AIAnalysisService) log(entry logger.LogEntry) {
	if s.Logger != nil {
		s.Logger.Log(entry)
	}
}

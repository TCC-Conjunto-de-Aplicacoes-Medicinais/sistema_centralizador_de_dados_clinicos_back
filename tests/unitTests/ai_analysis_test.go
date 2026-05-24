package unitTests

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	userHttp "github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/services/users/core/http"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/services/users/core/services"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/logger"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/models"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// mockRoundTripper intercepts HTTP requests for mock testing.
type mockRoundTripper struct {
	roundTripFunc func(req *http.Request) (*http.Response, error)
}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.roundTripFunc(req)
}

// -------------------------------------------------------------------
// Service Tests
// -------------------------------------------------------------------

func TestNewAIAnalysisService(t *testing.T) {
	appLogger := logger.NewLogger(nil)
	service := services.NewAIAnalysisService(nil, "mock-api-key", appLogger)

	assert.NotNil(t, service)
	assert.Equal(t, "mock-api-key", service.GeminiAPIKey)
	assert.NotNil(t, service.HTTPClient)
	assert.Equal(t, appLogger, service.Logger)
}

func TestAIAnalysisService_Analyze_NoApiKey(t *testing.T) {
	appLogger := logger.NewLogger(nil)
	service := services.NewAIAnalysisService(nil, "", appLogger)

	req := models.AIAnalysisRequest{Query: "test query"}
	res, err := service.Analyze("user-1", req)

	assert.Error(t, err)
	assert.Nil(t, res)
	assert.Contains(t, err.Error(), "chave da API Gemini não configurada")
}

func TestAIAnalysisService_Analyze_Success(t *testing.T) {
	appLogger := logger.NewLogger(nil)
	service := services.NewAIAnalysisService(nil, "mock-api-key", appLogger)

	// Mock successful response from Gemini
	mockRespJSON := `{
		"candidates": [
			{
				"content": {
					"parts": [
						{
							"text": "Esta é uma análise médica simulada para fins de teste."
						}
					]
				}
			}
		]
	}`

	service.HTTPClient = &http.Client{
		Transport: &mockRoundTripper{
			roundTripFunc: func(req *http.Request) (*http.Response, error) {
				assert.Equal(t, "POST", req.Method)
				assert.Contains(t, req.URL.String(), "gemini-2.0-flash")
				assert.Equal(t, "mock-api-key", req.URL.Query().Get("key"))

				bodyBytes, err := io.ReadAll(req.Body)
				assert.NoError(t, err)
				assert.Contains(t, string(bodyBytes), "test query")

				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(mockRespJSON)),
					Header:     make(http.Header),
				}, nil
			},
		},
	}

	req := models.AIAnalysisRequest{Query: "test query"}
	res, err := service.Analyze("user-1", req)

	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, "Esta é uma análise médica simulada para fins de teste.", res.Analysis)
	assert.Contains(t, res.Disclaimer, "AVISO IMPORTANTE")
}

func TestAIAnalysisService_Analyze_ShortResponseWarning(t *testing.T) {
	appLogger := logger.NewLogger(nil)
	service := services.NewAIAnalysisService(nil, "mock-api-key", appLogger)

	mockRespJSON := `{
		"candidates": [
			{
				"content": {
					"parts": [
						{
							"text": "Curta"
						}
					]
				}
			}
		]
	}`

	service.HTTPClient = &http.Client{
		Transport: &mockRoundTripper{
			roundTripFunc: func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(mockRespJSON)),
					Header:     make(http.Header),
				}, nil
			},
		},
	}

	req := models.AIAnalysisRequest{Query: "test query"}
	res, err := service.Analyze("user-1", req)

	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, "Curta", res.Analysis)
}

func TestAIAnalysisService_Analyze_HttpError(t *testing.T) {
	appLogger := logger.NewLogger(nil)
	service := services.NewAIAnalysisService(nil, "mock-api-key", appLogger)

	service.HTTPClient = &http.Client{
		Transport: &mockRoundTripper{
			roundTripFunc: func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusInternalServerError,
					Body:       io.NopCloser(strings.NewReader("Internal Server Error")),
					Header:     make(http.Header),
				}, nil
			},
		},
	}

	req := models.AIAnalysisRequest{Query: "test query"}
	res, err := service.Analyze("user-1", req)

	assert.Error(t, err)
	assert.Nil(t, res)
	assert.Contains(t, err.Error(), "API Gemini retornou status 500")
}

func TestAIAnalysisService_Analyze_ApiErrorPayload(t *testing.T) {
	appLogger := logger.NewLogger(nil)
	service := services.NewAIAnalysisService(nil, "mock-api-key", appLogger)

	mockRespJSON := `{
		"error": {
			"code": 400,
			"message": "API key not valid"
		}
	}`

	service.HTTPClient = &http.Client{
		Transport: &mockRoundTripper{
			roundTripFunc: func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(mockRespJSON)),
					Header:     make(http.Header),
				}, nil
			},
		},
	}

	req := models.AIAnalysisRequest{Query: "test query"}
	res, err := service.Analyze("user-1", req)

	assert.Error(t, err)
	assert.Nil(t, res)
	assert.Contains(t, err.Error(), "API key not valid")
}

func TestAIAnalysisService_Analyze_EmptyResponseCandidates(t *testing.T) {
	appLogger := logger.NewLogger(nil)
	service := services.NewAIAnalysisService(nil, "mock-api-key", appLogger)

	mockRespJSON := `{
		"candidates": []
	}`

	service.HTTPClient = &http.Client{
		Transport: &mockRoundTripper{
			roundTripFunc: func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(mockRespJSON)),
					Header:     make(http.Header),
				}, nil
			},
		},
	}

	req := models.AIAnalysisRequest{Query: "test query"}
	res, err := service.Analyze("user-1", req)

	assert.Error(t, err)
	assert.Nil(t, res)
	assert.Contains(t, err.Error(), "não retornou nenhuma resposta")
}

func TestAIAnalysisService_Analyze_InvalidJson(t *testing.T) {
	appLogger := logger.NewLogger(nil)
	service := services.NewAIAnalysisService(nil, "mock-api-key", appLogger)

	service.HTTPClient = &http.Client{
		Transport: &mockRoundTripper{
			roundTripFunc: func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader("invalid { json")),
					Header:     make(http.Header),
				}, nil
			},
		},
	}

	req := models.AIAnalysisRequest{Query: "test query"}
	res, err := service.Analyze("user-1", req)

	assert.Error(t, err)
	assert.Nil(t, res)
	assert.Contains(t, err.Error(), "erro ao decodificar resposta da API")
}

func TestAIAnalysisService_Analyze_NetworkError(t *testing.T) {
	appLogger := logger.NewLogger(nil)
	service := services.NewAIAnalysisService(nil, "mock-api-key", appLogger)

	service.HTTPClient = &http.Client{
		Transport: &mockRoundTripper{
			roundTripFunc: func(req *http.Request) (*http.Response, error) {
				return nil, errors.New("network timeout")
			},
		},
	}

	req := models.AIAnalysisRequest{Query: "test query"}
	res, err := service.Analyze("user-1", req)

	assert.Error(t, err)
	assert.Nil(t, res)
	assert.Contains(t, err.Error(), "network timeout")
}

// -------------------------------------------------------------------
// Handler Tests
// -------------------------------------------------------------------

func TestAIAnalyze_Handler_ServiceNil(t *testing.T) {
	appLogger := logger.NewLogger(nil)
	userHandler := userHttp.NewUserHandler(nil, nil, nil, nil, nil, nil, nil, appLogger)

	gin.SetMode(gin.TestMode)
	router := gin.Default()
	router.POST("/api/users/ai/analyze", userHandler.AIAnalyze)

	payload := []byte(`{"query": "exame"}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/users/ai/analyze", bytes.NewBuffer(payload))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "serviço de IA indisponível")
}

func TestAIAnalyze_Handler_Unauthorized(t *testing.T) {
	appLogger := logger.NewLogger(nil)
	service := services.NewAIAnalysisService(nil, "mock-api-key", appLogger)
	userHandler := userHttp.NewUserHandler(nil, nil, nil, nil, nil, service, nil, appLogger)

	gin.SetMode(gin.TestMode)
	router := gin.Default()
	// No userID set in context
	router.POST("/api/users/ai/analyze", userHandler.AIAnalyze)

	payload := []byte(`{"query": "exame"}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/users/ai/analyze", bytes.NewBuffer(payload))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "usuário não identificado")
}

func TestAIAnalyze_Handler_BadRequest(t *testing.T) {
	appLogger := logger.NewLogger(nil)
	service := services.NewAIAnalysisService(nil, "mock-api-key", appLogger)
	userHandler := userHttp.NewUserHandler(nil, nil, nil, nil, nil, service, nil, appLogger)

	gin.SetMode(gin.TestMode)
	router := gin.Default()
	router.Use(func(c *gin.Context) {
		c.Set("userID", "user-uuid-1234")
		c.Next()
	})
	router.POST("/api/users/ai/analyze", userHandler.AIAnalyze)

	// Missing query field
	payload := []byte(`{}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/users/ai/analyze", bytes.NewBuffer(payload))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "error")
}

func TestAIAnalyze_Handler_Success(t *testing.T) {
	appLogger := logger.NewLogger(nil)
	service := services.NewAIAnalysisService(nil, "mock-api-key", appLogger)

	mockRespJSON := `{
		"candidates": [
			{
				"content": {
					"parts": [
						{
							"text": "Análise de exames concluída com sucesso."
						}
					]
				}
			}
		]
	}`

	service.HTTPClient = &http.Client{
		Transport: &mockRoundTripper{
			roundTripFunc: func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(mockRespJSON)),
					Header:     make(http.Header),
				}, nil
			},
		},
	}

	userHandler := userHttp.NewUserHandler(nil, nil, nil, nil, nil, service, nil, appLogger)

	gin.SetMode(gin.TestMode)
	router := gin.Default()
	router.Use(func(c *gin.Context) {
		c.Set("userID", "user-uuid-1234")
		c.Next()
	})
	router.POST("/api/users/ai/analyze", userHandler.AIAnalyze)

	payload := []byte(`{"query": "Meu exame deu hemoglobina 12"}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/users/ai/analyze", bytes.NewBuffer(payload))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Análise de exames concluída com sucesso.")
	assert.Contains(t, w.Body.String(), "AVISO IMPORTANTE")
}

func TestAIAnalyze_Handler_ServiceError(t *testing.T) {
	appLogger := logger.NewLogger(nil)
	service := services.NewAIAnalysisService(nil, "mock-api-key", appLogger)

	service.HTTPClient = &http.Client{
		Transport: &mockRoundTripper{
			roundTripFunc: func(req *http.Request) (*http.Response, error) {
				return nil, errors.New("gemini connection error")
			},
		},
	}

	userHandler := userHttp.NewUserHandler(nil, nil, nil, nil, nil, service, nil, appLogger)

	gin.SetMode(gin.TestMode)
	router := gin.Default()
	router.Use(func(c *gin.Context) {
		c.Set("userID", "user-uuid-1234")
		c.Next()
	})
	router.POST("/api/users/ai/analyze", userHandler.AIAnalyze)

	payload := []byte(`{"query": "Meu exame deu hemoglobina 12"}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/users/ai/analyze", bytes.NewBuffer(payload))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "erro ao processar análise")
}

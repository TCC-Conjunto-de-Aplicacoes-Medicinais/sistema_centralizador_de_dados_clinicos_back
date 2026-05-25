package unitTests

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	userHttp "github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/services/users/core/http"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/services/users/core/services"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/config"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/logger"
	"github.com/gin-gonic/gin"
	"github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// setupMockMinIO cria um cliente MinIO mockado que aponta para um mock HTTP server.
func setupMockMinIO(t *testing.T) (*httptest.Server, *config.MinIOClient) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Logf("Mock MinIO received request: %s %s?%s", r.Method, r.URL.Path, r.URL.RawQuery)

		// 1. GetBucketLocation request
		if r.Method == http.MethodGet && (r.URL.Path == "/openhealth-app-exams" || r.URL.Path == "/openhealth-app-exams/") {
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><LocationConstraint xmlns="http://s3.amazonaws.com/doc/2006-03-01/">us-east-1</LocationConstraint>`))
			return
		}

		// 2. Object Metadata (HEAD)
		if r.Method == http.MethodHead {
			w.Header().Set("ETag", `"mock-etag"`)
			w.Header().Set("Last-Modified", "Sun, 24 May 2026 20:00:00 GMT")
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Content-Length", "12")
			w.WriteHeader(http.StatusOK)
			return
		}

		// 3. Object Content (GET)
		if r.Method == http.MethodGet {
			w.Header().Set("ETag", `"mock-etag"`)
			w.Header().Set("Last-Modified", "Sun, 24 May 2026 20:00:00 GMT")
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Content-Length", "12")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("mock content"))
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))

	// O endpoint no cliente MinIO deve apontar para o host/port do mock server sem o protocolo http://
	endpoint := strings.Replace(server.URL, "http://", "", 1)

	client, err := minio.New(endpoint, &minio.Options{
		Secure: false,
	})
	assert.NoError(t, err)

	return server, &config.MinIOClient{
		Client:     client,
		BucketName: "openhealth-app-exams",
	}
}

func TestExamService_GetExamFile_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      db,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	assert.NoError(t, err)

	minioServer, minioClient := setupMockMinIO(t)
	defer minioServer.Close()

	service := services.NewExamService(gormDB, minioClient, "http://localhost:8002")

	mock.ExpectQuery("SELECT \\* FROM `exam` WHERE id = \\?").
		WithArgs("exam-uuid-123", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "patient_id", "exam_type", "link_bucket"}).
			AddRow("exam-uuid-123", "patient-uuid-123", "blood-test", "http://localhost:8002/api/exams/file/exam-uuid-123/test.pdf"))

	stream, contentType, size, err := service.GetExamFile(context.Background(), "patient-uuid-123", "exam-uuid-123", "test.pdf")
	assert.NoError(t, err)
	assert.NotNil(t, stream)
	assert.Equal(t, "application/octet-stream", contentType)
	assert.Equal(t, int64(12), size)

	content, err := io.ReadAll(stream)
	assert.NoError(t, err)
	assert.Equal(t, "mock content", string(content))
	stream.Close()

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestExamService_GetExamFile_AccessDenied(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      db,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	assert.NoError(t, err)

	minioServer, minioClient := setupMockMinIO(t)
	defer minioServer.Close()

	service := services.NewExamService(gormDB, minioClient, "http://localhost:8002")

	// Mock DB query com dono diferente
	mock.ExpectQuery("SELECT \\* FROM `exam` WHERE id = \\?").
		WithArgs("exam-uuid-123", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "patient_id", "exam_type"}).
			AddRow("exam-uuid-123", "other-patient-uuid", "blood-test"))

	stream, contentType, size, err := service.GetExamFile(context.Background(), "patient-uuid-123", "exam-uuid-123", "test.pdf")
	assert.Error(t, err)
	assert.Nil(t, stream)
	assert.Contains(t, err.Error(), "acesso negado")
	assert.Equal(t, int64(0), size)
	assert.Equal(t, "", contentType)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetExamFile_Handler_Forbidden(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      db,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	assert.NoError(t, err)

	minioServer, minioClient := setupMockMinIO(t)
	defer minioServer.Close()

	examService := services.NewExamService(gormDB, minioClient, "http://localhost:8002")
	appLogger := logger.NewLogger(nil)
	examHandler := userHttp.NewExamHandler(examService, appLogger)

	gin.SetMode(gin.TestMode)
	router := gin.Default()
	router.Use(func(c *gin.Context) {
		c.Set("userID", "patient-uuid-123") // Logged in user
		c.Next()
	})
	router.GET("/api/exams/file/:id/:filename", examHandler.GetExamFile)

	// Mock DB query retornando dono diferente
	mock.ExpectQuery("SELECT \\* FROM `exam` WHERE id = \\?").
		WithArgs("exam-uuid-123", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "patient_id", "exam_type"}).
			AddRow("exam-uuid-123", "different-patient-uuid", "blood-test"))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/exams/file/exam-uuid-123/test.pdf", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "acesso negado")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestExamService_GetExams_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      db,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	assert.NoError(t, err)

	service := services.NewExamService(gormDB, nil, "http://localhost:8002")

	mock.ExpectQuery("SELECT \\* FROM `exam` WHERE \\(patient_id = \\? AND flag_active = \\?\\) AND `exam`.`deleted_at` IS NULL").
		WithArgs("patient-uuid-123", true).
		WillReturnRows(sqlmock.NewRows([]string{"id", "patient_id", "exam_type"}).
			AddRow("exam-1", "patient-uuid-123", "blood-test").
			AddRow("exam-2", "patient-uuid-123", "urine-test"))

	exams, err := service.GetExams(context.Background(), "patient-uuid-123")
	assert.NoError(t, err)
	assert.Len(t, exams, 2)
	assert.Equal(t, "exam-1", exams[0].Id)
	assert.Equal(t, "exam-2", exams[1].Id)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestExamService_GetExamByID_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      db,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	assert.NoError(t, err)

	service := services.NewExamService(gormDB, nil, "http://localhost:8002")

	mock.ExpectQuery("SELECT \\* FROM `exam` WHERE \\(id = \\? AND flag_active = \\?\\) AND `exam`.`deleted_at` IS NULL").
		WithArgs("exam-uuid-123", true, sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "patient_id", "exam_type"}).
			AddRow("exam-uuid-123", "patient-uuid-123", "blood-test"))

	exam, err := service.GetExamByID(context.Background(), "patient-uuid-123", "exam-uuid-123")
	assert.NoError(t, err)
	assert.NotNil(t, exam)
	assert.Equal(t, "exam-uuid-123", exam.Id)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetExams_Handler_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      db,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	assert.NoError(t, err)

	examService := services.NewExamService(gormDB, nil, "http://localhost:8002")
	appLogger := logger.NewLogger(nil)
	examHandler := userHttp.NewExamHandler(examService, appLogger)

	gin.SetMode(gin.TestMode)
	router := gin.Default()
	router.Use(func(c *gin.Context) {
		c.Set("userID", "patient-uuid-123")
		c.Next()
	})
	router.GET("/api/exams", examHandler.GetExams)

	mock.ExpectQuery("SELECT \\* FROM `exam` WHERE \\(patient_id = \\? AND flag_active = \\?\\) AND `exam`.`deleted_at` IS NULL").
		WithArgs("patient-uuid-123", true).
		WillReturnRows(sqlmock.NewRows([]string{"id", "patient_id", "exam_type"}).
			AddRow("exam-1", "patient-uuid-123", "blood-test"))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/exams", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "exam-1")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestExamService_DeleteExam_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      db,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	assert.NoError(t, err)

	service := services.NewExamService(gormDB, nil, "http://localhost:8002")

	// First query finds the exam
	mock.ExpectQuery("SELECT \\* FROM `exam` WHERE \\(id = \\? AND flag_active = \\?\\) AND `exam`.`deleted_at` IS NULL").
		WithArgs("exam-uuid-123", true, sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "patient_id"}).AddRow("exam-uuid-123", "patient-uuid-123"))

	// Second query/exec updates the deleted_at field (soft delete)
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `exam` SET `deleted_at`=\\? WHERE `exam`.`id` = \\?").
		WithArgs(sqlmock.AnyArg(), "exam-uuid-123").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err = service.DeleteExam(context.Background(), "patient-uuid-123", "exam-uuid-123")
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestExamService_DeleteExam_AccessDenied(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      db,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	assert.NoError(t, err)

	service := services.NewExamService(gormDB, nil, "http://localhost:8002")

	// First query finds the exam but owned by someone else
	mock.ExpectQuery("SELECT \\* FROM `exam` WHERE \\(id = \\? AND flag_active = \\?\\) AND `exam`.`deleted_at` IS NULL").
		WithArgs("exam-uuid-123", true, sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "patient_id"}).AddRow("exam-uuid-123", "other-patient-uuid"))

	err = service.DeleteExam(context.Background(), "patient-uuid-123", "exam-uuid-123")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "acesso negado")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteExam_Handler_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      db,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	assert.NoError(t, err)

	examService := services.NewExamService(gormDB, nil, "http://localhost:8002")
	appLogger := logger.NewLogger(nil)
	examHandler := userHttp.NewExamHandler(examService, appLogger)

	gin.SetMode(gin.TestMode)
	router := gin.Default()
	router.Use(func(c *gin.Context) {
		c.Set("userID", "patient-uuid-123")
		c.Next()
	})
	router.DELETE("/api/exams/:id", examHandler.DeleteExam)

	// Mock DB query & delete
	mock.ExpectQuery("SELECT \\* FROM `exam` WHERE \\(id = \\? AND flag_active = \\?\\) AND `exam`.`deleted_at` IS NULL").
		WithArgs("exam-uuid-123", true, sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "patient_id"}).AddRow("exam-uuid-123", "patient-uuid-123"))

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `exam` SET `deleted_at`=\\? WHERE `exam`.`id` = \\?").
		WithArgs(sqlmock.AnyArg(), "exam-uuid-123").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	w := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/api/exams/exam-uuid-123", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Exame deletado com sucesso!")
	assert.NoError(t, mock.ExpectationsWereMet())
}

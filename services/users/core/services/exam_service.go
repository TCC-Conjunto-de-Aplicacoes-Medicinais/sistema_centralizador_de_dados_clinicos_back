package services

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"time"

	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/config"
	"github.com/TCC-Conjunto-de-Aplicacoes-Medicinais/sistema_centralizador_de_dados_clinicos_back/shared/database"
	"github.com/gocql/gocql"
	"github.com/minio/minio-go/v7"
	"gorm.io/gorm"
)

type ExamService struct {
	DB      *gorm.DB
	MinIO   *config.MinIOClient
	BaseURL string
}

func NewExamService(db *gorm.DB, minioClient *config.MinIOClient, baseURL string) *ExamService {
	return &ExamService{
		DB:      db,
		MinIO:   minioClient,
		BaseURL: baseURL,
	}
}

// UploadExam salva o arquivo no MinIO e registra as metadados do exame no MariaDB.
func (s *ExamService) UploadExam(
	ctx context.Context,
	patientID string,
	file io.Reader,
	filename string,
	fileSize int64,
	contentType string,
	examDate time.Time,
	examType string,
	institution *string,
	examResult *string,
) (*database.Exam, error) {
	examID := gocql.TimeUUID().String()
	
	// Define o nome do objeto usando a estrutura: patient_id/exam_type/exam_id_filename
	objectName := fmt.Sprintf("%s/%s/%s_%s", patientID, examType, examID, filename)

	// Faz o upload para o MinIO
	_, err := s.MinIO.Client.PutObject(ctx, s.MinIO.BucketName, objectName, file, fileSize, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return nil, fmt.Errorf("falha ao enviar arquivo para o MinIO: %w", err)
	}

	// Link para recuperar o arquivo passando pelo endpoint autenticado do backend
	linkBucket := fmt.Sprintf("%s/api/exams/file/%s/%s", s.BaseURL, examID, filename)
	mockedCassandraID := gocql.TimeUUID().String() // Mock do Cassandra ID

	exam := &database.Exam{
		Id:          examID,
		PatientId:   patientID,
		LinkBucket:  linkBucket,
		IdCassandra: mockedCassandraID,
		Date:        examDate,
		ExamType:    examType,
		Institution: institution,
		ExamResult:  examResult,
		FlagActive:  true,
	}

	if err := s.DB.WithContext(ctx).Create(exam).Error; err != nil {
		// Se falhar ao salvar no banco, remove do MinIO para manter consistência
		_ = s.MinIO.Client.RemoveObject(ctx, s.MinIO.BucketName, objectName, minio.RemoveObjectOptions{})
		return nil, fmt.Errorf("falha ao salvar metadados do exame no banco: %w", err)
	}

	return exam, nil
}

// GetExamFile verifica a permissão do usuário e retorna o stream do arquivo a partir do MinIO.
func (s *ExamService) GetExamFile(
	ctx context.Context,
	patientID string,
	examID string,
	filename string,
) (io.ReadCloser, string, int64, error) {
	var exam database.Exam
	if err := s.DB.WithContext(ctx).First(&exam, "id = ?", examID).Error; err != nil {
		return nil, "", 0, fmt.Errorf("exame não encontrado: %w", err)
	}

	// Restringe o acesso apenas ao dono do exame (PatientId)
	if exam.PatientId != patientID {
		return nil, "", 0, fmt.Errorf("acesso negado ao arquivo do exame: usuário não autorizado")
	}

	// Tenta encontrar o arquivo no MinIO buscando por diferentes variações do nome do arquivo
	// (original, unescaped e escaped) para suportar variações de URL-encoding.
	possibleFilenames := []string{filename}
	if decoded, err := url.PathUnescape(filename); err == nil && decoded != filename {
		possibleFilenames = append(possibleFilenames, decoded)
	}
	if encoded := url.PathEscape(filename); encoded != filename {
		possibleFilenames = append(possibleFilenames, encoded)
	}
	if queryEncoded := url.QueryEscape(filename); queryEncoded != filename {
		possibleFilenames = append(possibleFilenames, queryEncoded)
	}

	var objectName string
	found := false

	for _, fn := range possibleFilenames {
		// 1. Novo caminho estruturado: patient_id/exam_type/exam_id_filename
		path1 := fmt.Sprintf("%s/%s/%s_%s", exam.PatientId, exam.ExamType, exam.Id, fn)
		if _, err := s.MinIO.Client.StatObject(ctx, s.MinIO.BucketName, path1, minio.StatObjectOptions{}); err == nil {
			objectName = path1
			found = true
			break
		}

		// 2. Caminho antigo de fallback: patient_id/exam_id_filename
		path2 := fmt.Sprintf("%s/%s_%s", exam.PatientId, exam.Id, fn)
		if _, err := s.MinIO.Client.StatObject(ctx, s.MinIO.BucketName, path2, minio.StatObjectOptions{}); err == nil {
			objectName = path2
			found = true
			break
		}
	}

	// Caso não encontre por nenhuma variação (para que GetObject/Stat retorne o erro correto), define a padrão
	if !found {
		objectName = fmt.Sprintf("%s/%s/%s_%s", exam.PatientId, exam.ExamType, exam.Id, filename)
	}

	object, err := s.MinIO.Client.GetObject(ctx, s.MinIO.BucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		return nil, "", 0, fmt.Errorf("falha ao obter arquivo do MinIO: %w", err)
	}

	stat, err := object.Stat()
	if err != nil {
		object.Close()
		return nil, "", 0, fmt.Errorf("falha ao obter metadados do objeto no MinIO: %w", err)
	}

	return object, stat.ContentType, stat.Size, nil
}

// GetExams retorna todos os exames ativos de um paciente ordenados por data decrescente.
func (s *ExamService) GetExams(ctx context.Context, patientID string) ([]database.Exam, error) {
	var exams []database.Exam
	err := s.DB.WithContext(ctx).
		Where("patient_id = ? AND flag_active = ?", patientID, true).
		Order("date desc").
		Find(&exams).Error
	if err != nil {
		return nil, fmt.Errorf("falha ao buscar exames do paciente no banco: %w", err)
	}
	return exams, nil
}

// GetExamByID retorna os detalhes de um exame específico se pertencer ao paciente informado.
func (s *ExamService) GetExamByID(ctx context.Context, patientID string, examID string) (*database.Exam, error) {
	var exam database.Exam
	err := s.DB.WithContext(ctx).
		Where("id = ? AND flag_active = ?", examID, true).
		First(&exam).Error
	if err != nil {
		return nil, fmt.Errorf("exame não encontrado: %w", err)
	}

	if exam.PatientId != patientID {
		return nil, fmt.Errorf("acesso negado ao exame: usuário não autorizado")
	}

	return &exam, nil
}

// DeleteExam realiza o soft delete de um exame, se pertencer ao paciente informado.
func (s *ExamService) DeleteExam(ctx context.Context, patientID string, examID string) error {
	var exam database.Exam
	err := s.DB.WithContext(ctx).
		Where("id = ? AND flag_active = ?", examID, true).
		First(&exam).Error
	if err != nil {
		return fmt.Errorf("exame não encontrado: %w", err)
	}

	if exam.PatientId != patientID {
		return fmt.Errorf("acesso negado ao exame: usuário não autorizado")
	}

	// Deleta o registro (soft delete automático por causa do gorm.Model)
	if err := s.DB.WithContext(ctx).Delete(&exam).Error; err != nil {
		return fmt.Errorf("falha ao deletar exame: %w", err)
	}

	return nil
}

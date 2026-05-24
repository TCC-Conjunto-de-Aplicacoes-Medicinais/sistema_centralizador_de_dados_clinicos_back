package config

import (
	"log"
	"os"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type MinIOClient struct {
	Client     *minio.Client
	BucketName string
}

func MinIOConnect() *MinIOClient {
	endpoint := os.Getenv("MINIO_ENDPOINT")
	accessKeyID := os.Getenv("MINIO_ROOT_USER")
	secretAccessKey := os.Getenv("MINIO_ROOT_PASSWORD")
	bucketName := os.Getenv("MINIO_BUCKET_NAME")

	if endpoint == "" {
		endpoint = "localhost:9000"
	}
	if accessKeyID == "" {
		accessKeyID = "minioadmin"
	}
	if secretAccessKey == "" {
		secretAccessKey = "minioadmin"
	}
	if bucketName == "" {
		bucketName = "openhealth-app-exams"
	}

	// Inicializa o cliente MinIO
	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: false,
	})
	if err != nil {
		log.Fatalf("❌ Erro ao conectar no MinIO: %v", err)
	}

	log.Println("✅ Conexão com MinIO estabelecida com sucesso!")

	return &MinIOClient{
		Client:     minioClient,
		BucketName: bucketName,
	}
}

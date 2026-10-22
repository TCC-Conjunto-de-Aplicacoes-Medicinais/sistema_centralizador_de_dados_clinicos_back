package config

import (
	"context"
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

	secure := true
	if os.Getenv("MINIO_SECURE") == "false" {
		secure = false
	}

	// Inicializa o cliente MinIO
	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: secure,
	})
	if err != nil {
		log.Fatalf("❌ Erro ao conectar no MinIO: %v", err)
	}

	log.Println("✅ Conexão com MinIO estabelecida com sucesso!")

	// Verifica e cria o bucket automaticamente caso ainda não exista
	ctx := context.Background()
	exists, errBucket := minioClient.BucketExists(ctx, bucketName)
	if errBucket != nil {
		log.Printf("⚠️ Aviso ao verificar bucket '%s' no MinIO: %v", bucketName, errBucket)
	} else if !exists {
		if err := minioClient.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{}); err != nil {
			log.Printf("⚠️ Erro ao criar bucket '%s' no MinIO: %v", bucketName, err)
		} else {
			log.Printf("✅ Bucket '%s' criado com sucesso no MinIO!", bucketName)
		}
	} else {
		log.Printf("✅ Bucket '%s' pronto no MinIO!", bucketName)
	}

	return &MinIOClient{
		Client:     minioClient,
		BucketName: bucketName,
	}
}

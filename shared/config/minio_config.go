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

	// Inicializa o cliente MinIO
	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: false,
	})
	if err != nil {
		log.Fatalf("❌ Erro ao conectar no MinIO: %v", err)
	}

	log.Println("✅ Conexão com MinIO estabelecida com sucesso!")

	// Cria o bucket se não existir
	ctx := context.Background()
	err = minioClient.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{})
	if err != nil {
		exists, errBucketExists := minioClient.BucketExists(ctx, bucketName)
		if errBucketExists == nil && exists {
			log.Printf("Bucket %s já existe\n", bucketName)
		} else {
			log.Fatalf("❌ Erro ao criar bucket no MinIO: %v", err)
		}
	} else {
		log.Printf("Bucket %s criado com sucesso\n", bucketName)
	}

	return &MinIOClient{
		Client:     minioClient,
		BucketName: bucketName,
	}
}

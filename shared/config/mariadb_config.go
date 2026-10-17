package config

import (
	"fmt"
	"log"
	"os"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func MariaDBConnect() *gorm.DB {
	user := os.Getenv("MARIADB_USER")
	if user == "" {
		user = os.Getenv("DB_USER")
	}
	pass := os.Getenv("MARIADB_PASSWORD")
	if pass == "" {
		pass = os.Getenv("DB_PASSWORD")
	}
	host := os.Getenv("MARIADB_HOST")
	if host == "" {
		host = os.Getenv("DB_HOST")
	}
	port := os.Getenv("MARIADB_PORT")
	if port == "" {
		port = os.Getenv("DB_PORT")
	}
	if port == "" {
		port = "3306"
	}
	dbname := os.Getenv("MARIADB_DB")
	if dbname == "" {
		dbname = os.Getenv("DB_NAME")
	}
	if dbname == "" {
		dbname = "pohinc_central"
	}

	if host == "" {
		log.Fatal("❌ Erro crítico: Configure o HOST do MariaDB (MARIADB_HOST ou DB_HOST) no arquivo .env!")
	}

	// O formato de conexão DSN do MySQL serve perfeitamente para o MariaDB
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		user, pass, host, port, dbname)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("❌ Erro ao conectar no MariaDB: %v", err)
	}

	log.Println("✅ Conexão com MariaDB estabelecida com sucesso!")
	
	// Aqui você pode adicionar opções de AutoMigrate de tabelas no futuro
	// db.AutoMigrate(&SuaStructAqui{})

	return db
}

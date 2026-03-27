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
	pass := os.Getenv("MARIADB_PASSWORD")
	host := os.Getenv("MARIADB_HOST")
	port := os.Getenv("MARIADB_PORT")
	dbname := os.Getenv("MARIADB_DB")

	if host == "" || dbname == "" {
		log.Fatal("❌ Erro crítico: Configure as variáveis do MariaDB no arquivo .env!")
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

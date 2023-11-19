package database

import (
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/ngolik/expense-service/model"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func ConnectDatabase() {
    // Загрузка переменных окружения из файла .env
    if err := godotenv.Load(); err != nil {
        log.Fatal("Error loading .env file")
    }

    // Получение значений переменных окружения
    dbUser := os.Getenv("DB_USER")
    dbPassword := os.Getenv("DB_PASSWORD")
    dbName := os.Getenv("DB_NAME")
    dbSSLMode := os.Getenv("DB_SSLMODE")

    // Формирование строки подключения
    connectionString := "user=" + dbUser + " dbname=" + dbName + " password=" + dbPassword + " sslmode=" + dbSSLMode

    // Открытие соединения с базой данных с использованием gorm.Open
    db, err := gorm.Open(postgres.Open(connectionString), &gorm.Config{})
    if err != nil {
        log.Fatal("failed to connect database")
    }

    // Включение логирования запросов (опционально)
    db.Logger.LogMode(3) // Уровень логирования. 3 - логирование всех запросов

	err = db.AutoMigrate(&model.Expense{})
	if err != nil {
		log.Fatal("failed to auto-migrate database schema")
	}

    // Сохранение указателя на базу данных для последующего использования
    DB = db
}
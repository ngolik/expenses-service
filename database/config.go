package database

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/ngolik/expense-service/model"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

// DatabaseConfig holds the configuration for the database.
type DatabaseConfig struct {
	DBUser     string
	DBPassword string
	DBName     string
}

// ConnectDatabase connects to the database using the provided configuration.
func ConnectDatabase(config DatabaseConfig) error {
	// Формирование строки подключения
	connectionString := fmt.Sprintf("user=%s dbname=%s password=%s",
		config.DBUser, config.DBName, config.DBPassword)

	// Открытие соединения с базой данных с использованием gorm.Open
	db, err := gorm.Open(postgres.Open(connectionString), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("failed to connect to the database: %v", err)
	}
	// Log database connection details
	log.Printf("Connected to the database: %s", connectionString)

	// Включение логирования запросов (опционально)
	db.Logger.LogMode(3) // Уровень логирования. Info - логирование всех запросов

	// Сохранение указателя на базу данных для последующего использования
	DB = db

	return nil
}

// MigrateDatabase performs auto-migration for the database schema.
func MigrateDatabase() error {
	// AutoMigrate here for better control
	err := DB.AutoMigrate(&model.Expense{})
	if err != nil {
		return fmt.Errorf("failed to auto-migrate database schema: %v", err)
	}
	log.Println("Database schema migration successful")
	return nil
}

func GetDatabaseConfigFromEnv() DatabaseConfig {
	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	}
	return DatabaseConfig{
		DBUser:    os.Getenv("DB_USER"),
		DBPassword: os.Getenv("DB_PASSWORD"),
		DBName:     os.Getenv("DB_NAME"),
	}
}

func InitializeDatabase() {
	// Load database configuration from .env
	databaseConfig := GetDatabaseConfigFromEnv()

	err := ConnectDatabase(databaseConfig)
	if err != nil {
		log.Fatalf("Failed to connect to the database: %v", err)
		return
	}

	err = MigrateDatabase()
	if err != nil {
		log.Fatalf("Failed to migrate database schema: %v", err)
		return
	}
	log.Println("Database initialization completed successfully")
}

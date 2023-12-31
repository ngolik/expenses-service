package database

import (
	"fmt"
	"log"
	"os"
	"strconv"

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
	DBHost     string
	DBPort     int
}

// ConnectDatabase connects to the database using the provided configuration.
func ConnectDatabase(config DatabaseConfig) error {
	// Формирование строки подключения
	connectionString := fmt.Sprintf("host=cashflow-postgres user=%s dbname=%s password=%s port=%d sslmode=disable",
		config.DBUser, config.DBName, config.DBPassword, config.DBPort)

	// Открытие соединения с базой данных с использованием gorm.Open
	db, err := gorm.Open(postgres.Open(connectionString), &gorm.Config{})
	if err != nil {
		fmt.Println("Connection details: ", connectionString)
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
	envFile := os.Getenv("EXPENSE_SERVICE_ENV_FILE_PATH")
	if envFile == "" {
		log.Println("envFile not found")
		envFile = ".env"
	}

	if err := godotenv.Load(envFile); err != nil {
		log.Fatal(err, " Error loading db props .env file")
	}
	port, err := strconv.Atoi(os.Getenv("POSTGRES_PORT"))
	if err != nil {
		fmt.Errorf("Error converting string to int")
	}
	return DatabaseConfig{
		DBUser:     os.Getenv("POSTGRES_USER"),
		DBPassword: os.Getenv("POSTGRES_PASSWORD"),
		DBName:     os.Getenv("POSTGRES_NAME"),
		DBHost:     os.Getenv("POSTGRES_HOST"),
		DBPort:     port,
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

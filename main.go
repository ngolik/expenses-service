package main

import (
	"github.com/ngolik/expense-service/api"
	"github.com/ngolik/expense-service/database"
	"os"

	"github.com/gin-gonic/gin"
)

func main() {
	api.EurekaClientConfig()
	database.InitializeDatabase()

	router := gin.Default()
	api.SetupRoutes(router)

	err := router.Run(":" + os.Getenv("SERVER_PORT"))
	if err != nil {
		return
	}
}

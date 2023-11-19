package main

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/ngolik/expense-service/database"
	"github.com/ngolik/expense-service/model"
	"github.com/ngolik/expense-service/service"
	eureka "github.com/ArthurHlt/go-eureka-client/eureka"
)

func EurekaClientConfig() {

    // Initialize the Eureka client with server URL
    client := eureka.NewClient([]string{"http://localhost:8761/eureka"})
    fmt.Println("Printing Client Details..")
    fmt.Println(client)
    // eureka.NewInstanceInfo(hostname, appName, ipAddr, port, securePort, ttl, secure)
    instance := eureka.NewInstanceInfo(
        "localhost",
        "EXPENSES-API",
        "127.0.0.1",
        8083,
        30,
        false,
    )

    // Register the instance with the Eureka server
    client.RegisterInstance("EXPENSES-API", instance)
    client.SendHeartbeat(instance.App, instance.HostName)
    fmt.Println("Printing Instance Details...")
    fmt.Println(client.GetInstance(instance.App, instance.HostName))
}

func main() {
	EurekaClientConfig()

	// Initialize the database connection
	database.ConnectDatabase()

	// Create a Gin router
	router := gin.Default()

	// Group all routes under "/expenses"
	expensesGroup := router.Group("/expenses")

	// Define the route to add an expense under the "/expenses" group
	expensesGroup.POST("/add", func(c *gin.Context) {
		var expense model.Expense

		// Bind the JSON request body to the Expense struct
		if err := c.BindJSON(&expense); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}

		// Call the AddExpense function to insert the expense into the database
		err := service.AddExpense(expense)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		// Respond with a success message
		c.JSON(200, gin.H{"message": "Expense added successfully"})
	})

	// Add more routes under the "/expenses" group as needed

	// Run the application on port 8083
	router.Run(":8083")
}
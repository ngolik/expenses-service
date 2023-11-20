package api

import (
	"github.com/gin-gonic/gin"
	"github.com/ngolik/expense-service/model"
	"github.com/ngolik/expense-service/service"
)

func AddExpenseHandler(c *gin.Context) {
	var expense model.Expense
	if err := c.BindJSON(&expense); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	err := service.AddExpense(expense)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "Expense added successfully"})
}

// GetExpensesHandler retrieves all expenses and returns them as JSON.
func GetExpensesHandler(c *gin.Context) {
	expenses, err := service.GetAllExpenses()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, expenses)
}

func GetHealthCheck(c *gin.Context) {
	if isHealthy() {
		c.JSON(200, gin.H{"status": "UP"})
	} else {
		c.JSON(500, gin.H{"status": "unhealthy"})
	}
}

func isHealthy() bool {
	// Your health check logic goes here
	// You may check the database connection, dependencies, etc.
	// For simplicity, we'll just return true
	return true
}

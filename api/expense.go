package api

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/ngolik/expense-service/model"
	"github.com/ngolik/expense-service/service"
	"gorm.io/gorm"
)

func AddExpenseHandler(c *gin.Context) {
	var expense model.Expense
	if err := c.BindJSON(&expense); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	err := service.AddExpense(expense)
	if err != nil {
		if errors.Is(err, service.ErrUnknownUser) {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
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

// GetExpenseByIdHandler retrieves a single expense by id and returns it as JSON.
func GetExpenseByIdHandler(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid id"})
		return
	}

	expense, err := service.GetExpenseById(uint(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(404, gin.H{"error": "expense not found"})
			return
		}
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, expense)
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

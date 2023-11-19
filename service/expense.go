package service

import (
	"github.com/ngolik/expense-service/database"
	"github.com/ngolik/expense-service/model"
)

func AddExpense(expense model.Expense) error {
	// Assuming DB is a global variable that holds the GORM DB connection
	result := database.DB.Create(&expense)

	if result.Error != nil {
		return result.Error
	}

	return nil
}
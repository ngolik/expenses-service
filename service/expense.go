package service

import (
	"github.com/ngolik/expense-service/database"
	"github.com/ngolik/expense-service/model"
)

func AddExpense(expense model.Expense) error {
	result := database.DB.Create(&expense)
	if result.Error != nil {
		return result.Error
	}

	return nil
}

// GetAllExpenses retrieves all expenses from the database.
func GetAllExpenses() ([]model.Expense, error) {
	var expenses []model.Expense

	result := database.DB.Find(&expenses)
	if result.Error != nil {
		return nil, result.Error
	}
	return expenses, nil
}

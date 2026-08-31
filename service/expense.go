package service

import (
	"errors"

	"github.com/ngolik/expense-service/database"
	"github.com/ngolik/expense-service/model"
	"gorm.io/gorm"
)

// ErrUnknownUser is returned when a create request sets UserID to a value
// that does not match any existing user.
var ErrUnknownUser = errors.New("unknown user")

// userExpenseRepository is the seam AddExpense/GetExpenseById use to reach
// storage, so unit tests can swap in an in-memory fake instead of a live
// database connection.
type userExpenseRepository interface {
	UserExists(userID int) (bool, error)
	CreateExpense(expense model.Expense) error
	FindExpenseByID(id uint) (model.Expense, error)
}

// Repo is the active repository implementation. Production code uses the
// GORM-backed default; tests reassign it to an in-memory fake.
var Repo userExpenseRepository = gormRepository{}

type gormRepository struct{}

func (gormRepository) UserExists(userID int) (bool, error) {
	var user model.User
	result := database.DB.First(&user, userID)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if result.Error != nil {
		return false, result.Error
	}
	return true, nil
}

func (gormRepository) CreateExpense(expense model.Expense) error {
	return database.DB.Create(&expense).Error
}

func (gormRepository) FindExpenseByID(id uint) (model.Expense, error) {
	var expense model.Expense
	result := database.DB.First(&expense, id)
	if result.Error != nil {
		return model.Expense{}, result.Error
	}
	return expense, nil
}

func AddExpense(expense model.Expense) error {
	if expense.UserID != 0 {
		exists, err := Repo.UserExists(expense.UserID)
		if err != nil {
			return err
		}
		if !exists {
			return ErrUnknownUser
		}
	}

	return Repo.CreateExpense(expense)
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

// GetExpenseById retrieves a single expense by its primary key.
func GetExpenseById(id uint) (model.Expense, error) {
	return Repo.FindExpenseByID(id)
}

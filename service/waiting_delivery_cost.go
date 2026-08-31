package service

import (
	"fmt"

	"github.com/ngolik/expense-service/database"
	"github.com/ngolik/expense-service/model"
)

// UserValidator checks whether a user id corresponds to a real, existing
// system user. Defined here (the consumer) rather than in authclient so
// this package never needs to import HTTP machinery - authclient.
// HTTPUserValidator satisfies this interface structurally, and tests can
// supply a fake instead.
type UserValidator interface {
	UserExists(userID int) (bool, error)
}

// ExpenseRepository abstracts persistence for wait-cost expense records so
// AddWaitingDeliveryCost/GetWaitingDeliveryCost can be unit-tested without a
// live database. The existing AddExpense/GetAllExpenses functions are
// intentionally left untouched, using database.DB directly as before.
type ExpenseRepository interface {
	Create(expense *model.Expense) error
	FindByID(id uint) (*model.Expense, error)
}

// gormExpenseRepository is the production ExpenseRepository, backed by GORM
// via database.DB.
type gormExpenseRepository struct{}

func (gormExpenseRepository) Create(expense *model.Expense) error {
	return database.DB.Create(expense).Error
}

func (gormExpenseRepository) FindByID(id uint) (*model.Expense, error) {
	var expense model.Expense
	if err := database.DB.First(&expense, id).Error; err != nil {
		return nil, err
	}
	return &expense, nil
}

// DefaultExpenseRepository is the GORM-backed repository used in production.
var DefaultExpenseRepository ExpenseRepository = gormExpenseRepository{}

// ValidationError indicates a rejected wait-cost creation request due to
// missing/invalid input or an unknown user id - as opposed to a downstream
// failure such as auth-service being unreachable or a DB error. Handlers map
// this to HTTP 400.
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

func newValidationError(format string, args ...interface{}) *ValidationError {
	return &ValidationError{Message: fmt.Sprintf(format, args...)}
}

// UpstreamError indicates the auth-service call itself failed (network
// error, timeout, unexpected status) rather than confirming the user id is
// unknown. Handlers map this to HTTP 502, distinct from a 400 validation
// failure, because the caller's input was never actually judged invalid.
type UpstreamError struct {
	Message string
	Err     error
}

func (e *UpstreamError) Error() string {
	return e.Message
}

func (e *UpstreamError) Unwrap() error {
	return e.Err
}

func newUpstreamError(err error) *UpstreamError {
	return &UpstreamError{Message: fmt.Sprintf("failed to validate user with auth-service: %v", err), Err: err}
}

// AddWaitingDeliveryCost creates a wait-cost expense record. All three of
// DeliveryID, UserID, and Amount must be present (non-zero), and UserID must
// be confirmed by validator to correspond to a real, existing user. This is
// a new, additional creation path - it does not affect AddExpense/the
// existing POST /expenses/rest/add contract in any way.
//
// expense is taken by pointer (not value) so that repo.Create's assignment
// of the generated ID is visible to the caller after this function returns -
// a value parameter would only ever populate a local copy's ID, leaving the
// caller with a record it has no way to retrieve afterwards.
func AddWaitingDeliveryCost(expense *model.Expense, validator UserValidator, repo ExpenseRepository) error {
	if expense.DeliveryID == 0 {
		return newValidationError("deliveryId is required")
	}
	if expense.UserID == 0 {
		return newValidationError("userId is required")
	}
	if expense.Amount == 0 {
		return newValidationError("amount is required")
	}

	exists, err := validator.UserExists(expense.UserID)
	if err != nil {
		return newUpstreamError(err)
	}
	if !exists {
		return newValidationError("userId %d does not correspond to an existing user", expense.UserID)
	}

	if err := repo.Create(expense); err != nil {
		return err
	}
	return nil
}

// GetWaitingDeliveryCost retrieves one wait-cost expense record by id.
// Returns an error satisfying errors.Is(err, gorm.ErrRecordNotFound) when no
// such record exists.
func GetWaitingDeliveryCost(id uint, repo ExpenseRepository) (*model.Expense, error) {
	expense, err := repo.FindByID(id)
	if err != nil {
		return nil, err
	}
	return expense, nil
}

// Ensure the sentinel error types satisfy the standard error interface as
// expected by errors.As at call sites (see api/waiting_delivery_cost.go).
var (
	_ error = (*ValidationError)(nil)
	_ error = (*UpstreamError)(nil)
)

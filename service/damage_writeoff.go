package service

import (
	"github.com/ngolik/expense-service/model"
)

// AddDamageWriteOff creates a damage-writeoff expense record. All three of
// DeliveryID, UserID, and Amount must be present (non-zero), and UserID must
// be confirmed by validator to correspond to a real, existing user - the
// same validation AddWaitingDeliveryCost applies. This is a new, additional
// creation path - it does not affect AddExpense/AddWaitingDeliveryCost or
// their existing contracts in any way.
//
// expense.IsDamageWriteOff is set true before persistence so this record is
// distinguishable from a wait-cost record sharing the same DeliveryID (AC8) -
// AddWaitingDeliveryCost never sets this field, so it stays false there.
//
// expense is taken by pointer (not value) so that repo.Create's assignment
// of the generated ID is visible to the caller after this function returns -
// a value parameter would only ever populate a local copy's ID, leaving the
// caller with a record it has no way to retrieve afterwards.
func AddDamageWriteOff(expense *model.Expense, validator UserValidator, repo ExpenseRepository) error {
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

	expense.IsDamageWriteOff = true

	if err := repo.Create(expense); err != nil {
		return err
	}
	return nil
}

// GetDamageWriteOff retrieves one damage-writeoff expense record by id.
// Returns an error satisfying errors.Is(err, gorm.ErrRecordNotFound) when no
// such record exists.
func GetDamageWriteOff(id uint, repo ExpenseRepository) (*model.Expense, error) {
	expense, err := repo.FindByID(id)
	if err != nil {
		return nil, err
	}
	return expense, nil
}

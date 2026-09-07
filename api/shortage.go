package api

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/ngolik/expense-service/authclient"
	"github.com/ngolik/expense-service/model"
	"github.com/ngolik/expense-service/service"
	"gorm.io/gorm"
)

// shortageValidator and shortageRepo are the real, production wiring for the
// new shortage path - a live call to auth-service and the real GORM-backed
// repository, respectively. Both are package-level vars precisely so
// same-package tests can substitute fakes (see shortage_test.go) without a
// live auth-service or database.
var (
	shortageValidator service.UserValidator     = authclient.NewHTTPUserValidator()
	shortageRepo      service.ExpenseRepository = service.DefaultExpenseRepository
)

// ShortageResponse is the finance-facing read shape for a shortage record:
// at least the delivery id, operator/user id, and amount.
type ShortageResponse struct {
	ID         uint    `json:"id"`
	DeliveryID int     `json:"deliveryId"`
	UserID     int     `json:"userId"`
	Amount     float64 `json:"amount"`
}

// AddShortageHandler creates a new shortage record and returns it (including
// the generated id) so the caller can retrieve it afterwards via
// GetShortageHandler. This is a new, additional creation path alongside
// AddExpenseHandler/AddWaitingDeliveryCostHandler/AddDamageWriteOffHandler -
// it requires deliveryId, userId (validated against auth-service), and
// amount, and rejects (4xx) when any is missing/invalid. It has no effect on
// any of those existing handlers' behavior.
func AddShortageHandler(c *gin.Context) {
	var expense model.Expense
	if err := c.BindJSON(&expense); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	err := service.AddShortage(&expense, shortageValidator, shortageRepo)
	if err != nil {
		var validationErr *service.ValidationError
		var upstreamErr *service.UpstreamError
		switch {
		case errors.As(err, &validationErr):
			c.JSON(400, gin.H{"error": err.Error()})
		case errors.As(err, &upstreamErr):
			c.JSON(502, gin.H{"error": err.Error()})
		default:
			c.JSON(500, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(200, ShortageResponse{
		ID:         expense.ID,
		DeliveryID: expense.DeliveryID,
		UserID:     expense.UserID,
		Amount:     expense.Amount,
	})
}

// GetShortageHandler retrieves one shortage record by id - the
// finance-facing read for this story. Returns the delivery id, user id, and
// amount.
func GetShortageHandler(c *gin.Context) {
	idParam := c.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil {
		c.JSON(400, gin.H{"error": "id must be a positive integer"})
		return
	}

	expense, err := service.GetShortage(uint(id), shortageRepo)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(404, gin.H{"error": "shortage record not found"})
			return
		}
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, ShortageResponse{
		ID:         expense.ID,
		DeliveryID: expense.DeliveryID,
		UserID:     expense.UserID,
		Amount:     expense.Amount,
	})
}

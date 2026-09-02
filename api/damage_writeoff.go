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

// damageWriteOffValidator and damageWriteOffRepo are the real, production
// wiring for the new damage-writeoff path - a live call to auth-service and
// the real GORM-backed repository, respectively. Both are package-level vars
// precisely so same-package tests can substitute fakes (see
// damage_writeoff_test.go) without a live auth-service or database.
var (
	damageWriteOffValidator service.UserValidator     = authclient.NewHTTPUserValidator()
	damageWriteOffRepo      service.ExpenseRepository = service.DefaultExpenseRepository
)

// DamageWriteOffResponse is the finance-facing read shape for a
// damage-writeoff record: at least the delivery id, operator/user id, and
// amount.
type DamageWriteOffResponse struct {
	ID         uint    `json:"id"`
	DeliveryID int     `json:"deliveryId"`
	UserID     int     `json:"userId"`
	Amount     float64 `json:"amount"`
}

// AddDamageWriteOffHandler creates a new damage-writeoff record and returns
// it (including the generated id) so the caller can retrieve it afterwards
// via GetDamageWriteOffHandler. This is a new, additional creation path
// alongside AddExpenseHandler/AddWaitingDeliveryCostHandler - it requires
// deliveryId, userId (validated against auth-service), and amount, and
// rejects (4xx) when any is missing/invalid. It has no effect on either of
// those existing handlers' behavior.
func AddDamageWriteOffHandler(c *gin.Context) {
	var expense model.Expense
	if err := c.BindJSON(&expense); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	err := service.AddDamageWriteOff(&expense, damageWriteOffValidator, damageWriteOffRepo)
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

	c.JSON(200, DamageWriteOffResponse{
		ID:         expense.ID,
		DeliveryID: expense.DeliveryID,
		UserID:     expense.UserID,
		Amount:     expense.Amount,
	})
}

// GetDamageWriteOffHandler retrieves one damage-writeoff record by id - the
// finance-facing read for this story. Returns the delivery id, user id, and
// amount.
func GetDamageWriteOffHandler(c *gin.Context) {
	idParam := c.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil {
		c.JSON(400, gin.H{"error": "id must be a positive integer"})
		return
	}

	expense, err := service.GetDamageWriteOff(uint(id), damageWriteOffRepo)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(404, gin.H{"error": "damage write-off record not found"})
			return
		}
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, DamageWriteOffResponse{
		ID:         expense.ID,
		DeliveryID: expense.DeliveryID,
		UserID:     expense.UserID,
		Amount:     expense.Amount,
	})
}

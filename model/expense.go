package model

import (
	"gorm.io/gorm"
	"time"
)

type User struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	UserName string `json:"username"`
	Password string `json:"password"`
}

// Expense represents an individual expense entry.
type Expense struct {
	gorm.Model
	Description string
	Amount      float64
	Category    string
	Date        time.Time
	UserID      int
	Healthy     bool
	// DeliveryID links this expense to a waiting delivery (arrival-service's
	// Arrival.id). Zero value means "not linked to a delivery" - this is the
	// value left by the existing general-purpose POST /expenses/rest/add
	// path, which never sets it. Only the waiting-delivery-cost creation
	// path (service.AddWaitingDeliveryCost) requires it to be non-zero.
	DeliveryID int
}

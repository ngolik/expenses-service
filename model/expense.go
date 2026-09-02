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
	// path, which never sets it. Both the waiting-delivery-cost creation path
	// (service.AddWaitingDeliveryCost) and the damage-writeoff creation path
	// (service.AddDamageWriteOff) require it to be non-zero.
	DeliveryID int
	// IsDamageWriteOff distinguishes a damage write-off record from a
	// wait-cost record sharing the same DeliveryID/UserID/Amount fields -
	// finance must not see the two conflated for the same delivery (AC8).
	// Zero value (false) is what every existing record has - the
	// general-purpose POST /expenses/rest/add path and the
	// waiting-delivery-cost creation path (service.AddWaitingDeliveryCost)
	// both leave it false. Only the damage-writeoff creation path
	// (service.AddDamageWriteOff) sets it true.
	IsDamageWriteOff bool
}

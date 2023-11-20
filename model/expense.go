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
}

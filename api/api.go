package api

import (
	"github.com/gin-gonic/gin"
)

func SetupRoutes(router *gin.Engine) {
	expensesGroup := router.Group("/expenses/rest")
	expensesGroup.POST("/add", AddExpenseHandler)
	expensesGroup.GET("/all", GetExpensesHandler)
}

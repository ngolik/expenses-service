package api

import (
	"github.com/gin-gonic/gin"
)

func SetupRoutes(router *gin.Engine) {
	expensesGroup := router.Group("/expenses/rest")
	expensesGroup.POST("/add", AddExpenseHandler)
	expensesGroup.GET("/all", GetExpensesHandler)
	expensesGroup.POST("/waiting-cost", AddWaitingDeliveryCostHandler)
	expensesGroup.GET("/waiting-cost/:id", GetWaitingDeliveryCostHandler)
	expensesGroup.GET("/:id", GetExpenseByIdHandler)
}

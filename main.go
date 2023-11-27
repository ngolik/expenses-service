package main

import (
	"github.com/ngolik/expense-service/api"
	"github.com/ngolik/expense-service/database"
	"os"
	"os/signal"
	"time"

	"github.com/gin-gonic/gin"
)

func main() {
	database.InitializeDatabase()
	eurekaRegister := api.BuildFargoInstance()
	eurekaRegister.Register()

	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt)

	router := gin.Default()
	api.SetupRoutes(router)

	err := router.Run(":8083")
	if err != nil {
		return
	}

	go func() {
		select {
		case signal := <-c:
			_ = signal
			time.Sleep(4 * time.Second)
			eurekaRegister.Deregister()
			os.Exit(1)
		}
	}()
}

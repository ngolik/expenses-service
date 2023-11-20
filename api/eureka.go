package api

import (
	"fmt"
	"github.com/ArthurHlt/go-eureka-client/eureka"
	"github.com/joho/godotenv"
	"log"
	"os"
	"strconv"
	"time"
)

func createEurekaClient(eurekaHost, eurekaPort string) (*eureka.Client, error) {
	client := eureka.NewClient([]string{fmt.Sprintf("http://%s:%s/eureka", eurekaHost, eurekaPort)})
	fmt.Println("Printing Client Details..")
	fmt.Println(client)
	return client, nil
}

func createInstanceInfo(appName string) (*eureka.InstanceInfo, error) {
	serverPort, err := strconv.Atoi(os.Getenv("SERVER_PORT"))
	if err != nil {
		return nil, err
	}
	instance := eureka.NewInstanceInfo(
		os.Getenv("EUREKA_HOST"),
		os.Getenv("APP_NAME"),
		"127.0.0.1",
		serverPort,
		30,
		false,
	)
	// Example health check URL
	instance.HealthCheckUrl = fmt.Sprintf("http://%s:8083/health", instance.HostName)

	fmt.Println(appName)
	return instance, nil
}

func registerWithEureka(client *eureka.Client, appName string, instance *eureka.InstanceInfo) error {
	if err := client.RegisterInstance(appName, instance); err != nil {
		return fmt.Errorf("failed to register instance with Eureka: %v", err)
	}
	return nil
}

func sendHeartbeat(client *eureka.Client, appName, hostName string) error {
	if err := client.SendHeartbeat(appName, hostName); err != nil {
		return fmt.Errorf("failed to send heartbeat to Eureka: %v", err)
	}
	return nil
}

func EurekaClientConfig() {
	// Load environment variables from .env file
	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	}

	eurekaHost := os.Getenv("EUREKA_HOST")
	eurekaPort := os.Getenv("EUREKA_PORT")
	appName := os.Getenv("APP_NAME")

	client, err := createEurekaClient(eurekaHost, eurekaPort)
	if err != nil {
		log.Printf("Failed to create Eureka client: %v", err)
		return
	}

	instance, err := createInstanceInfo(appName)
	if err != nil {
		log.Printf("Failed to create Eureka instance info: %v", err)
		return
	}

	err = registerWithEureka(client, appName, instance)
	if err != nil {
		log.Printf("Failed to register instance with Eureka: %v", err)
		return
	}

	err = sendHeartbeat(client, appName, instance.HostName)
	if err != nil {
		log.Printf("Failed to send heartbeat to Eureka: %v", err)
		return
	}

	// Periodically send heartbeats to renew the lease
	go func() {
		for {
			time.Sleep(5 * time.Second) // Adjust the interval as needed
			client.SendHeartbeat(instance.App, instance.HostName)
			fmt.Println("Ping")
		}
	}()

	fmt.Println("Printing Instance Details...")
	fmt.Println(client.GetInstance(instance.App, instance.HostName))
}

package api

import (
	// "errors"
	"fmt"
	log "log"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/go-kit/kit/sd/eureka"
	kitlog "github.com/go-kit/log"
	"github.com/hudl/fargo"
	"github.com/joho/godotenv"
)

func buildFargoInstanceBody(appName, status string) *fargo.Instance {
	// Load environment variables from .env file
	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	}

	ipAddress, err := externalIP()
	if err != nil {
		fmt.Println(err)
	}

	stringPort := os.Getenv("SERVER_PORT")

	port, err := strconv.Atoi(stringPort)
	if err != nil {
		kitlog.ErrMissingValue.Error()
	}

	return &fargo.Instance{
		InstanceId:        ipAddress + ":" + stringPort,
		HostName:          ipAddress,
		App:               strings.ToUpper(appName),
		IPAddr:            ipAddress,
		VipAddress:        appName,
		SecureVipAddress:  appName,
		Status:            fargo.StatusType(status),
		Overriddenstatus:  "UNKNOWN",
		Port:              port,
		PortEnabled:       true,
		SecurePort:        8443,
		SecurePortEnabled: false,
		HomePageUrl:       "http://" + ipAddress + ":" + strconv.Itoa(port) + "/",
		StatusPageUrl:     "http://" + ipAddress + ":" + strconv.Itoa(port) + "/status",
		HealthCheckUrl:    "http://" + ipAddress + ":" + strconv.Itoa(port) + "/health",

		CountryId: 0,
		DataCenterInfo: fargo.DataCenterInfo{
			Name: "MyOwn", Class: "com.netflix.appinfo.InstanceInfo$DefaultDataCenterInfo",
		},
		LeaseInfo: fargo.LeaseInfo{},
		Metadata:  fargo.InstanceMetadata{},
		UniqueID:  nil,
	}
}

// BuildFargoInstance build a Fargo Instance and return eureka.Registrar
func BuildFargoInstance() eureka.Registrar {
	eurekaAddr := os.Getenv("EUREKA_URL")
	if eurekaAddr == "" {
		fmt.Println("EUREKA_SERVER_URL is not set")
	}

	logger := kitlog.NewLogfmtLogger(os.Stderr)
	logger = kitlog.With(logger, "ts", kitlog.DefaultTimestamp)

	var fargoConfig fargo.Config
	fargoConfig.Eureka.ServiceUrls = []string{eurekaAddr}
	fargoConfig.Eureka.PollIntervalSeconds = 1

	fargoConnection := fargo.NewConnFromConfig(fargoConfig)
	fInstance := buildFargoInstanceBody(os.Getenv("APP_NAME"), "UP")
	return *eureka.NewRegistrar(&fargoConnection, fInstance, kitlog.With(logger, "component", "registrar"))
}

// aux func to get external ip from
// aux func to get external ip from
func externalIP() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue // interface down
		}
		if iface.Flags&net.FlagLoopback != 0 {
			continue // loopback interface
		}
		addrs, err := iface.Addrs()
		if err != nil {
			return "", err
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			ip = ip.To4()
			if ip == nil {
				continue // not an ipv4 address
			}
			return ip.String(), nil
		}
	}
	return os.Getenv("IP"), nil // fallback to localhost if no suitable IP is found
}

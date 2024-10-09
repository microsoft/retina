package config

import (
	"log"
	"os"
	"strconv"
	"time"
)

const (
	defaultHTTPPort      = 8080
	defaultTCPPort       = 8085
	defaultUDPPort       = 8086
	defaultBurstVolume   = 1
	defaultBurstInterval = 500 * time.Millisecond

	EnvHTTPPort      = "HTTP_PORT"
	EnvTCPPort       = "TCP_PORT"
	EnvUDPPort       = "UDP_PORT"
	EnvBurstVolume   = "BURST_VOLUME"
	EnvBurstInterval = "BURST_INTERVAL_MS"
)

// just basic homebrew config, no viper/cobra to keep binary tiny
type KapingerConfig struct {
	BurstVolume   int
	BurstInterval time.Duration
	HTTPPort      int
	TCPPort       int
	UDPPort       int
}

// configmap later, but for now env is fine
func LoadConfigFromEnv() *KapingerConfig {
	k := &KapingerConfig{}
	var err error

	k.TCPPort, err = strconv.Atoi(os.Getenv(EnvTCPPort))
	if err != nil {
		k.TCPPort = defaultTCPPort
		log.Printf("%s not set, defaulting to port %d\n", EnvTCPPort, defaultTCPPort)
	}

	k.UDPPort, err = strconv.Atoi(os.Getenv(EnvUDPPort))
	if err != nil {
		k.UDPPort = defaultUDPPort
		log.Printf("%s not set, defaulting to port %d\n", EnvUDPPort, defaultUDPPort)
	}

	k.HTTPPort, err = strconv.Atoi(os.Getenv(EnvHTTPPort))
	if err != nil {
		k.HTTPPort = defaultHTTPPort
		log.Printf("%s not set, defaulting to port %d\n", EnvHTTPPort, defaultHTTPPort)
	}

	k.BurstVolume, err = strconv.Atoi(os.Getenv(EnvHTTPPort))
	if err != nil {
		k.BurstVolume = defaultBurstVolume
		log.Printf("%s not set, defaulting to %d\n", EnvBurstVolume, defaultBurstVolume)
	}

	burstInterval, err := strconv.Atoi(os.Getenv(EnvBurstInterval))
	if err != nil {
		k.BurstInterval = defaultBurstInterval
		log.Printf("%s not set, defaulting to %d\n", EnvBurstInterval, defaultBurstInterval)
	} else {
		k.BurstInterval = time.Duration(burstInterval) * time.Millisecond
	}

	return k
}

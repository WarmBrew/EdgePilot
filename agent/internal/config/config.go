package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	ServerURL         string
	AgentToken        string
	DeviceID          string
	Platform          string
	Arch              string
	LogLevel          string
	HeartbeatInterval int
}

func Load(envFile string) (*Config, error) {
	_ = godotenv.Load(envFile)

	heartbeatInterval := 30
	if v := os.Getenv("HEARTBEAT_INTERVAL"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			heartbeatInterval = parsed
		}
	}

	return &Config{
		ServerURL:         getEnvOrDefault("SERVER_URL", "wss://localhost:8080/ws/agent"),
		AgentToken:        getEnvOrDefault("AGENT_TOKEN", ""),
		DeviceID:          getEnvOrDefault("DEVICE_ID", ""),
		Platform:          getEnvOrDefault("PLATFORM", "linux"),
		Arch:              getEnvOrDefault("ARCH", "amd64"),
		LogLevel:          getEnvOrDefault("LOG_LEVEL", "info"),
		HeartbeatInterval: heartbeatInterval,
	}, nil
}

func getEnvOrDefault(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

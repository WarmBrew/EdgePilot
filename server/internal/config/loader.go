package config

import (
	"os"

	"github.com/joho/godotenv"
)

// Load initializes configuration from .env file and environment variables.
// It first loads the .env file using godotenv, then reads config via viper.
func Load(envFile string) error {
	if envFile == "" {
		envFile = ".env"
	}

	if _, err := os.Stat(envFile); err == nil {
		if err := godotenv.Load(envFile); err != nil {
			return err
		}
	}

	return InitConfig()
}

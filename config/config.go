package config

import (
	"fmt"
	"os"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	AppPort string

	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string

	LogLevel string
	AppEnv   string
}

// Load reads configuration from environment variables, applying sane defaults
// for local development. It never returns an error so the app can boot with
// defaults, but values can be overridden via the environment / .env file.
func Load() *Config {
	return &Config{
		AppPort: getEnv("APP_PORT", "3000"),

		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "3306"),
		DBUser:     getEnv("DB_USER", "root"),
		DBPassword: getEnv("DB_PASSWORD", "3499"),
		DBName:     getEnv("DB_NAME", "ainyx_users"),

		LogLevel: getEnv("LOG_LEVEL", "info"),
		AppEnv:   getEnv("APP_ENV", "development"),
	}
}

// DSN builds a MySQL data source name compatible with the go-sql-driver/mysql
// driver. parseTime=true is required so DATE/TIMESTAMP columns scan into
// time.Time (which sqlc-generated code expects).
func (c *Config) DSN() string {
	return fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?parseTime=true&loc=UTC&multiStatements=true",
		c.DBUser, c.DBPassword, c.DBHost, c.DBPort, c.DBName,
	)
}

// IsProduction reports whether the app is running in production mode.
func (c *Config) IsProduction() bool {
	return c.AppEnv == "production"
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

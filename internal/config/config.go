package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Database DatabaseConfig
	Server   ServerConfig
	JWT      JWTConfig
	Logging  LoggingConfig
}

type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

type ServerConfig struct {
	Port string
}

type JWTConfig struct {
	SigningKey string
	Issuer     string
	Expiry     time.Duration
}

type LoggingConfig struct {
	Level      string
	Format     string
	Filename   string
	MaxSize    int
	MaxBackups int
	MaxAge     int
	Compress   bool
}

func Load() *Config {
	return &Config{
		Database: DatabaseConfig{
			Host:     getEnv("POSTGRES_HOST", "localhost"),
			Port:     getEnv("POSTGRES_PORT", "5432"),
			User:     getEnv("POSTGRES_USER", "postgres"),
			Password: getEnv("POSTGRES_PASSWORD", ""),
			DBName:   getEnv("POSTGRES_DB", "postgres"),
			SSLMode:  getEnv("POSTGRES_SSL_MODE", "disable"),
		},
		Server: ServerConfig{
			Port: getEnv("SERVER_PORT", "8080"),
		},
		JWT: JWTConfig{
			SigningKey: getEnv("JWT_SIGNING_KEY", "default-signing-key-change-in-production"),
			Issuer:     getEnv("JWT_ISSUER", "campus-vault"),
			Expiry:     getEnvDuration("JWT_EXPIRY", 24*time.Hour),
		},
		Logging: LoggingConfig{
			Level:      getEnv("LOG_LEVEL", "info"),
			Format:     getEnv("LOG_FORMAT", "json"),
			Filename:   getEnv("LOG_FILENAME", "logs/app.log"),
			MaxSize:    getEnvAs("LOG_MAX_SIZE", 100, strconv.Atoi),
			MaxBackups: getEnvAs("LOG_MAX_BACKUPS", 3, strconv.Atoi),
			MaxAge:     getEnvAs("LOG_MAX_AGE", 28, strconv.Atoi),
			Compress:   getEnvAs("LOG_COMPRESS", true, strconv.ParseBool),
		},
	}
}

func (c *DatabaseConfig) ConnectionString() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.DBName, c.SSLMode,
	)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

func getEnvAs[T any](key string, defaultValue T, parser func(string) (T, error)) T {
	if value := os.Getenv(key); value != "" {
		if parsed, err := parser(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}
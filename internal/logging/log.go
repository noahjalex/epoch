package logging

import (
	"os"
	"strings"

	"github.com/sirupsen/logrus"
)

// Config holds logging configuration
type Config struct {
	Level       string
	Format      string
	Output      string
	HTTPLogging bool
}

// LoadConfig loads logging configuration from environment variables
func LoadConfig() *Config {
	return &Config{
		Level:       getEnv("EPOCH_LOG_LEVEL", "info"),
		Format:      getEnv("EPOCH_LOG_FORMAT", "text"),   // text or json
		Output:      getEnv("EPOCH_LOG_OUTPUT", "stdout"), // stdout, stderr, or file path
		HTTPLogging: getEnvBool("EPOCH_LOG_HTTP", true),   // Enable HTTP logging by default
	}
}

// Init creates and configures a new logger instance
func Init(config *Config) *logrus.Logger {
	logger := logrus.New()

	// Set log level
	level, err := logrus.ParseLevel(config.Level)
	if err != nil {
		logger.Warnf("Invalid log level '%s', defaulting to info", config.Level)
		level = logrus.InfoLevel
	}
	logger.SetLevel(level)

	// Set formatter
	switch strings.ToLower(config.Format) {
	case "json":
		logger.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: "2006-01-02T15:04:05.000Z07:00",
		})
	default:
		logger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: "2006-01-02T15:04:05.000Z07:00",
		})
	}

	// Set output
	switch strings.ToLower(config.Output) {
	case "stderr":
		logger.SetOutput(os.Stderr)
	case "stdout", "":
		logger.SetOutput(os.Stdout)
	default:
		// Assume it's a file path
		file, err := os.OpenFile(config.Output, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			logger.Warnf("Failed to open log file '%s', using stdout: %v", config.Output, err)
			logger.SetOutput(os.Stdout)
		} else {
			logger.SetOutput(file)
		}
	}

	logger.WithFields(logrus.Fields{
		"level":        config.Level,
		"format":       config.Format,
		"output":       config.Output,
		"http_logging": config.HTTPLogging,
	}).Info("Logger initialized")

	return logger
}

// getEnv gets an environment variable with a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvBool gets a boolean environment variable with a default value
func getEnvBool(key string, defaultValue bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return strings.ToLower(value) == "true" || value == "1"
}

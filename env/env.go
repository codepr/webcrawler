// Package env contains utilities to manage environemnt variables
package env

import (
	"os"
	"strconv"
)

// Simple helper function to read an environment variable or return a default value
func GetEnv(key string, defaultVal string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultVal
}

// Simple helper function to read an environment variable into an integer or return a default value
func GetEnvAsInt(key string, defaultVal int) int {
	valueStr := GetEnv(key, "")
	if value, err := strconv.Atoi(valueStr); err == nil {
		return value
	}
	return defaultVal
}

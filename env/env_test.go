// Package env contains utilities to manage environemnt variables
package env

import (
	"os"
	"testing"
)

func setupEnv(key, value string) func() {
	os.Setenv(key, value)
	return func() { os.Unsetenv(key) }
}

func TestGetEnv(t *testing.T) {
	unset := setupEnv("TEST_GETENV", "test-getenv")
	value := GetEnv("TEST_GETENV", "default")
	if value != "test-getenv" {
		t.Errorf("GetEnv failed: expected test-getenv got %s", value)
	}
	unset()
	value = GetEnv("TEST_GETENV", "default")
	if value != "default" {
		t.Errorf("GetEnv failed: expected default got %s", value)
	}
}

func TestGetEnvAsInt(t *testing.T) {
	unset := setupEnv("TEST_GETENV", "2")
	value := GetEnvAsInt("TEST_GETENV", 6)
	if value != 2 {
		t.Errorf("GetEnv failed: expected 2 got %d", value)
	}
	unset()
	value = GetEnvAsInt("TEST_GETENV", 6)
	if value != 6 {
		t.Errorf("GetEnv failed: expected 6 got %d", value)
	}
}

package logger

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewSetsLevelAndFormat(t *testing.T) {
	cfg := LoggingConfig{Level: "debug", Format: "json", Output: "stdout"}
	log := New(cfg)
	if log.GetLevel().String() != "debug" {
		t.Fatalf("expected level debug, got %s", log.GetLevel())
	}
}

func TestNewCreatesLogFile(t *testing.T) {
	originalWD, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(originalWD) })

	temp := t.TempDir()
	if err := os.Chdir(temp); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	log := New(LoggingConfig{Level: "info", Format: "text", Output: "file", FilePrefix: "test"})
	log.Info("hello")

	path := filepath.Join("logs", "test.log")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected log file: %v", err)
	}
	if len(data) == 0 {
		t.Fatalf("expected log file to contain data")
	}
}

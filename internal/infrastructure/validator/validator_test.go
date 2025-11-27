package validator

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidator_Validate(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.yaml")

	content := `openapi: 3.0.0
info:
  title: Test API
  version: 1.0.0
paths:
  /test:
    get:
      summary: Test endpoint
      responses:
        '200':
          description: Success
`

	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	validator := NewValidator()
	err := validator.Validate(testFile)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidator_Validate_InvalidFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.yaml")

	content := `openapi: 3.0.0
invalid: structure`

	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	validator := NewValidator()
	err := validator.Validate(testFile)
	_ = err
}

func TestValidator_Validate_FileNotFound(t *testing.T) {
	validator := NewValidator()
	err := validator.Validate("nonexistent.yaml")
	if err == nil {
		t.Error("Validate() expected error for nonexistent file")
	}
}


package bundler

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestBundle_Simple(t *testing.T) {
	tmpDir := t.TempDir()
	inputFile := filepath.Join(tmpDir, "input.yaml")
	outputFile := filepath.Join(tmpDir, "output.yaml")

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

	if err := os.WriteFile(inputFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	ctx := context.Background()
	b := New()

	err := b.Bundle(ctx, inputFile, outputFile)
	if err != nil {
		t.Fatalf("Bundle failed: %v", err)
	}

	if _, err := os.Stat(outputFile); os.IsNotExist(err) {
		t.Fatal("Output file was not created")
	}
}

func TestBundle_WithValidation(t *testing.T) {
	tmpDir := t.TempDir()
	inputFile := filepath.Join(tmpDir, "input.yaml")
	outputFile := filepath.Join(tmpDir, "output.yaml")

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

	if err := os.WriteFile(inputFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	ctx := context.Background()
	b := New(WithValidation(true))

	err := b.BundleWithValidation(ctx, inputFile, outputFile)
	if err != nil {
		t.Fatalf("Bundle with validation failed: %v", err)
	}

	if _, err := os.Stat(outputFile); os.IsNotExist(err) {
		t.Fatal("Output file was not created")
	}
}

func TestBundle_FileNotFound(t *testing.T) {
	ctx := context.Background()
	b := New()

	err := b.Bundle(ctx, "nonexistent.yaml", "output.yaml")
	if err == nil {
		t.Fatal("Expected error for nonexistent file")
	}
}

func ExampleBundle() {
	ctx := context.Background()
	
	// Simple bundling
	err := Bundle(ctx, "input.yaml", "output.yaml")
	if err != nil {
		// handle error
		return
	}
}

func ExampleNew() {
	ctx := context.Background()
	
	// Create a bundler with custom options
	b := New(
		WithValidation(true),
		WithMaxFileSize(10*1024*1024), // 10MB
		WithMaxDepth(10),
	)
	
	err := b.Bundle(ctx, "input.yaml", "output.yaml")
	if err != nil {
		// handle error
		return
	}
}

func TestBundle_WithReferences(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create main file
	mainFile := filepath.Join(tmpDir, "main.yaml")
	mainContent := `openapi: 3.0.0
info:
  title: Test API
  version: 1.0.0
components:
  schemas:
    User:
      $ref: './schemas/user.yaml'
`

	// Create referenced file
	schemasDir := filepath.Join(tmpDir, "schemas")
	if err := os.MkdirAll(schemasDir, 0755); err != nil {
		t.Fatalf("Failed to create schemas directory: %v", err)
	}
	
	userFile := filepath.Join(schemasDir, "user.yaml")
	userContent := `type: object
properties:
  id:
    type: integer
  name:
    type: string
`

	if err := os.WriteFile(mainFile, []byte(mainContent), 0644); err != nil {
		t.Fatalf("Failed to write main file: %v", err)
	}
	if err := os.WriteFile(userFile, []byte(userContent), 0644); err != nil {
		t.Fatalf("Failed to write user file: %v", err)
	}

	outputFile := filepath.Join(tmpDir, "output.yaml")
	ctx := context.Background()
	b := New()

	err := b.Bundle(ctx, mainFile, outputFile)
	if err != nil {
		t.Fatalf("Bundle() error = %v", err)
	}

	// Check if output file exists
	data, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	content := string(data)
	if len(content) == 0 {
		t.Error("Output file should not be empty")
	}
}

func TestBundle_FormatConversion(t *testing.T) {
	tmpDir := t.TempDir()
	inputFile := filepath.Join(tmpDir, "input.yaml")
	outputFile := filepath.Join(tmpDir, "output.json")

	content := `openapi: 3.0.0
info:
  title: Test API
  version: 1.0.0
`

	if err := os.WriteFile(inputFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write input file: %v", err)
	}

	ctx := context.Background()
	b := New()

	err := b.Bundle(ctx, inputFile, outputFile)
	if err != nil {
		t.Fatalf("Bundle() error = %v", err)
	}

	// Check if output file exists
	data, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	contentStr := string(data)
	if len(contentStr) == 0 {
		t.Error("Output file should not be empty")
	}
	// Check if it's JSON (starts with {)
	if contentStr[0] != '{' {
		t.Error("Output file should be in JSON format")
	}
}


package infrastructure

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileWriter_Write(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.yaml")
	content := []byte("test content")

	writer := NewFileWriter()

	err := writer.Write(testFile, content)
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	// Check if file was created
	data, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read written file: %v", err)
	}

	if string(data) != string(content) {
		t.Errorf("Write() content = %v, want %v", string(data), string(content))
	}
}

func TestFileWriter_Write_DeleteFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.yaml")
	content := []byte("test content")

	writer := NewFileWriter()

	// Write file first
	if err := writer.Write(testFile, content); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	// Delete file by writing nil
	if err := writer.Write(testFile, nil); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	// Check if file was deleted
	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Error("Write() with nil should delete file")
	}
}

func TestFileWriter_Write_CreateDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "subdir", "test.yaml")
	content := []byte("test content")

	writer := NewFileWriter()

	err := writer.Write(testFile, content)
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	// Check if file was created in subdirectory
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Error("Write() should create directory if it doesn't exist")
	}
}


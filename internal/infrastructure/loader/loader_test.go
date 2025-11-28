package loader

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestFileLoader_Load_LocalFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.yaml")
	content := []byte("test content")

	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	loader := NewFileLoader()
	ctx := context.Background()

	data, err := loader.Load(ctx, testFile)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if string(data) != string(content) {
		t.Errorf("Load() = %v, want %v", string(data), string(content))
	}
}

func TestFileLoader_Load_FileNotFound(t *testing.T) {
	loader := NewFileLoader()
	ctx := context.Background()

	_, err := loader.Load(ctx, "nonexistent.yaml")
	if err == nil {
		t.Error("Load() expected error for nonexistent file")
	}
}

func TestFileLoader_Load_ContextCancelled(t *testing.T) {
	loader := NewFileLoader()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := loader.Load(ctx, "test.yaml")
	if err == nil {
		t.Error("Load() expected error for cancelled context")
	}
}

func TestFileLoader_Load_RelativePath(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.yaml")
	content := []byte("test content")

	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	loader := NewFileLoader()
	ctx := context.Background()

	oldDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	data, err := loader.Load(ctx, "test.yaml")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if string(data) != string(content) {
		t.Errorf("Load() = %v, want %v", string(data), string(content))
	}
}

func TestFileLoader_Load_PathTraversal(t *testing.T) {
	loader := NewFileLoader()
	ctx := context.Background()

	_, err := loader.Load(ctx, "../../../etc/passwd")
	if err == nil {
		t.Error("Load() should protect against path traversal")
	}
}

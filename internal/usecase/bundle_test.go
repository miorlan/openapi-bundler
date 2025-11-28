package usecase

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/miorlan/openapi-bundler/internal/domain"
)

// mockFileLoader is a mock implementation of FileLoader
type mockFileLoader struct {
	files map[string][]byte
}

func (m *mockFileLoader) Load(ctx context.Context, path string) ([]byte, error) {
	if data, ok := m.files[path]; ok {
		return data, nil
	}
	return nil, os.ErrNotExist
}

// mockFileWriter is a mock implementation of FileWriter
type mockFileWriter struct {
	files map[string][]byte
}

func (m *mockFileWriter) Write(path string, data []byte) error {
	if m.files == nil {
		m.files = make(map[string][]byte)
	}
	m.files[path] = data
	return nil
}

// mockParser is a mock implementation of Parser
type mockParser struct{}

func (m *mockParser) Unmarshal(data []byte, v interface{}, format domain.FileFormat) error {
	// Simple mock - just set a basic structure
	if m, ok := v.(*map[string]interface{}); ok {
		*m = map[string]interface{}{
			"openapi": "3.0.0",
			"info": map[string]interface{}{
				"title": "Test API",
			},
		}
	}
	return nil
}

func (m *mockParser) Marshal(v interface{}, format domain.FileFormat) ([]byte, error) {
	return []byte("mocked output"), nil
}

// mockReferenceResolver is a mock implementation of ReferenceResolver
type mockReferenceResolver struct{}

func (m *mockReferenceResolver) Resolve(ctx context.Context, ref string, basePath string, config domain.Config) (interface{}, error) {
	return nil, nil
}

func (m *mockReferenceResolver) ResolveAll(ctx context.Context, data map[string]interface{}, basePath string, config domain.Config) error {
	return nil
}

// mockValidator is a mock implementation of Validator
type mockValidator struct{}

func (m *mockValidator) Validate(filePath string) error {
	return nil
}

func TestBundleUseCase_Execute(t *testing.T) {
	tests := []struct {
		name    string
		setup   func() *BundleUseCase
		wantErr bool
	}{
		{
			name: "successful bundle",
			setup: func() *BundleUseCase {
				loader := &mockFileLoader{
					files: map[string][]byte{
						"input.yaml": []byte("openapi: 3.0.0"),
					},
				}
				writer := &mockFileWriter{}
				parser := &mockParser{}
				resolver := &mockReferenceResolver{}
				validator := &mockValidator{}

				return NewBundleUseCase(loader, writer, parser, resolver, validator)
			},
			wantErr: false,
		},
		{
			name: "file not found",
			setup: func() *BundleUseCase {
				loader := &mockFileLoader{files: make(map[string][]byte)}
				writer := &mockFileWriter{}
				parser := &mockParser{}
				resolver := &mockReferenceResolver{}
				validator := &mockValidator{}

				return NewBundleUseCase(loader, writer, parser, resolver, validator)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc := tt.setup()
			ctx := context.Background()
			tmpDir := t.TempDir()
			outputPath := filepath.Join(tmpDir, "output.yaml")

			inputPath := "input.yaml"
			if tt.name == "file not found" {
				inputPath = "nonexistent.yaml"
			}

			err := uc.Execute(ctx, inputPath, outputPath, Config{})
			if (err != nil) != tt.wantErr {
				t.Errorf("BundleUseCase.Execute() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetBasePath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "HTTP URL",
			path: "https://example.com/api/openapi.yaml",
			want: "https://example.com/api/",
		},
		{
			name: "HTTP URL without file",
			path: "https://example.com/api/",
			want: "https://example.com/api/",
		},
		{
			name: "local file",
			path: "/path/to/file.yaml",
			want: "/path/to",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getBasePath(tt.path)
			if got != tt.want {
				t.Errorf("getBasePath() = %v, want %v", got, tt.want)
			}
		})
	}
}


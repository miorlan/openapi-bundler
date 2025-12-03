package domain

import "context"

// Config contains resolver configuration
type Config struct {
	MaxFileSize int64
	MaxDepth    int
	Inline      bool
}

// FileLoader loads files from filesystem or URL
type FileLoader interface {
	Load(ctx context.Context, path string) ([]byte, error)
}

// FileWriter writes files to filesystem
type FileWriter interface {
	Write(path string, data []byte) error
}

// Validator validates OpenAPI specifications
type Validator interface {
	Validate(filePath string) error
}

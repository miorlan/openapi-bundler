package domain

import "context"

type Config struct {
	MaxFileSize int64
	MaxDepth    int
	Inline      bool
}

type FileLoader interface {
	Load(ctx context.Context, path string) ([]byte, error)
}

type FileWriter interface {
	Write(path string, data []byte) error
}

type Parser interface {
	Unmarshal(data []byte, v interface{}, format FileFormat) error
	Marshal(v interface{}, format FileFormat) ([]byte, error)
}

type Validator interface {
	Validate(filePath string) error
}

type ReferenceResolver interface {
	Resolve(ctx context.Context, ref string, basePath string, config Config) (interface{}, error)
	ResolveAll(ctx context.Context, data map[string]interface{}, basePath string, config Config) error
}


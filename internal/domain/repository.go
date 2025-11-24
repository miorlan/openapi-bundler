package domain

import "context"

// Repository интерфейсы для работы с внешними зависимостями

// Config holds configuration for reference resolution
type Config struct {
	MaxFileSize int64
	MaxDepth    int
}

// FileLoader загружает содержимое файла (локального или по HTTP)
type FileLoader interface {
	Load(ctx context.Context, path string) ([]byte, error)
}

// FileWriter записывает данные в файл
type FileWriter interface {
	Write(path string, data []byte) error
}

// Parser парсит и сериализует данные в различных форматах
type Parser interface {
	Unmarshal(data []byte, v interface{}, format FileFormat) error
	Marshal(v interface{}, format FileFormat) ([]byte, error)
}

// Validator валидирует OpenAPI спецификацию
type Validator interface {
	Validate(filePath string) error
}

// ReferenceResolver разрешает $ref ссылки
type ReferenceResolver interface {
	Resolve(ctx context.Context, ref string, basePath string, config Config) (interface{}, error)
	ResolveAll(ctx context.Context, data map[string]interface{}, basePath string, config Config) error
}


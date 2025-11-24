package usecase

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/miorlan/openapi-bundler/internal/domain"
)

// Config holds configuration for bundle execution
type Config struct {
	Validate    bool
	MaxFileSize int64
	MaxDepth    int
}

// BundleUseCase реализует бизнес-логику объединения OpenAPI спецификаций
type BundleUseCase struct {
	fileLoader        domain.FileLoader
	fileWriter        domain.FileWriter
	parser            domain.Parser
	referenceResolver domain.ReferenceResolver
	validator         domain.Validator
}

// NewBundleUseCase создает новый экземпляр BundleUseCase
func NewBundleUseCase(
	fileLoader domain.FileLoader,
	fileWriter domain.FileWriter,
	parser domain.Parser,
	referenceResolver domain.ReferenceResolver,
	validator domain.Validator,
) *BundleUseCase {
	return &BundleUseCase{
		fileLoader:        fileLoader,
		fileWriter:        fileWriter,
		parser:            parser,
		referenceResolver: referenceResolver,
		validator:         validator,
	}
}

// Execute выполняет объединение OpenAPI спецификации
func (uc *BundleUseCase) Execute(ctx context.Context, inputPath, outputPath string, config Config) error {
	// Проверяем контекст
	if ctx.Err() != nil {
		return ctx.Err()
	}

	// Определяем форматы
	inputFormat := domain.DetectFormat(inputPath)
	outputFormat := domain.DetectFormat(outputPath)
	if outputFormat == "" {
		outputFormat = inputFormat
	}

	// Загружаем входной файл
	data, err := uc.fileLoader.Load(ctx, inputPath)
	if err != nil {
		return fmt.Errorf("failed to load input file: %w", err)
	}

	// Проверяем размер файла
	if config.MaxFileSize > 0 && int64(len(data)) > config.MaxFileSize {
		return fmt.Errorf("file size %d exceeds maximum allowed size %d", len(data), config.MaxFileSize)
	}

	// Парсим входной файл
	var root map[string]interface{}
	if err := uc.parser.Unmarshal(data, &root, inputFormat); err != nil {
		return fmt.Errorf("failed to parse input file: %w", err)
	}

	// Определяем базовый путь
	basePath := getBasePath(inputPath)

	// Разрешаем все ссылки
	domainConfig := domain.Config{
		MaxFileSize: config.MaxFileSize,
		MaxDepth:    config.MaxDepth,
	}
	if err := uc.referenceResolver.ResolveAll(ctx, root, basePath, domainConfig); err != nil {
		return fmt.Errorf("failed to resolve references: %w", err)
	}

	// Сериализуем результат
	outputData, err := uc.parser.Marshal(root, outputFormat)
	if err != nil {
		return fmt.Errorf("failed to marshal result: %w", err)
	}

	// Записываем результат
	if err := uc.fileWriter.Write(outputPath, outputData); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	// Валидация (если требуется)
	if config.Validate {
		if err := uc.validator.Validate(outputPath); err != nil {
			// Удаляем файл при ошибке валидации
			_ = uc.fileWriter.Write(outputPath, nil)
			return fmt.Errorf("validation failed: %w", err)
		}
	}

	return nil
}

// getBasePath определяет базовый путь для разрешения ссылок
func getBasePath(path string) string {
	// HTTP/HTTPS URL
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		lastSlash := strings.LastIndex(path, "/")
		if lastSlash >= 0 {
			return path[:lastSlash+1]
		}
		return path + "/"
	}

	// Локальный файл - возвращаем директорию
	absPath, err := filepath.Abs(path)
	if err != nil {
		// Если не удалось получить абсолютный путь, используем относительный
		return filepath.Dir(path)
	}
	return filepath.Dir(filepath.Clean(absPath))
}


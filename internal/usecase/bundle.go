package usecase

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/miorlan/openapi-bundler/internal/domain"
)

type Config struct {
	Validate    bool
	MaxFileSize int64
	MaxDepth    int
}

type BundleUseCase struct {
	fileLoader        domain.FileLoader
	fileWriter        domain.FileWriter
	parser            domain.Parser
	referenceResolver domain.ReferenceResolver
	validator         domain.Validator
}

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

func (uc *BundleUseCase) Execute(ctx context.Context, inputPath, outputPath string, config Config) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	inputFormat := domain.DetectFormat(inputPath)
	outputFormat := domain.DetectFormat(outputPath)
	if outputFormat == "" {
		outputFormat = inputFormat
	}

	data, err := uc.fileLoader.Load(ctx, inputPath)
	if err != nil {
		return fmt.Errorf("failed to load input file: %w", err)
	}

	if config.MaxFileSize > 0 && int64(len(data)) > config.MaxFileSize {
		return fmt.Errorf("file size %d exceeds maximum allowed size %d", len(data), config.MaxFileSize)
	}

	var root map[string]interface{}
	if err := uc.parser.Unmarshal(data, &root, inputFormat); err != nil {
		return fmt.Errorf("failed to parse input file: %w", err)
	}

	basePath := getBasePath(inputPath)

	domainConfig := domain.Config{
		MaxFileSize: config.MaxFileSize,
		MaxDepth:    config.MaxDepth,
	}
	if err := uc.referenceResolver.ResolveAll(ctx, root, basePath, domainConfig); err != nil {
		return fmt.Errorf("failed to resolve references: %w", err)
	}

	outputData, err := uc.parser.Marshal(root, outputFormat)
	if err != nil {
		return fmt.Errorf("failed to marshal result: %w", err)
	}

	if err := uc.fileWriter.Write(outputPath, outputData); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	if config.Validate {
		if err := uc.validator.Validate(outputPath); err != nil {
			_ = uc.fileWriter.Write(outputPath, nil)
			return fmt.Errorf("validation failed: %w", err)
		}
	}

	return nil
}

func getBasePath(path string) string {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		lastSlash := strings.LastIndex(path, "/")
		if lastSlash >= 0 {
			return path[:lastSlash+1]
		}
		return path + "/"
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return filepath.Dir(path)
	}
	return filepath.Dir(filepath.Clean(absPath))
}


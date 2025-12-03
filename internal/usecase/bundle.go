package usecase

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/miorlan/openapi-bundler/internal/domain"
	"github.com/miorlan/openapi-bundler/internal/infrastructure/parser"
	"github.com/miorlan/openapi-bundler/internal/infrastructure/resolver"
)

// Config contains bundler configuration
type Config struct {
	Validate    bool
	MaxFileSize int64
	MaxDepth    int
	Inline      bool
}

// BundleUseCase bundles OpenAPI specs using yaml.Node to preserve order
type BundleUseCase struct {
	fileLoader domain.FileLoader
	fileWriter domain.FileWriter
	validator  domain.Validator
}

// NewBundleUseCase creates a new BundleUseCase
func NewBundleUseCase(
	fileLoader domain.FileLoader,
	fileWriter domain.FileWriter,
	validator domain.Validator,
) *BundleUseCase {
	return &BundleUseCase{
		fileLoader: fileLoader,
		fileWriter: fileWriter,
		validator:  validator,
	}
}

// Execute bundles the OpenAPI specification
func (uc *BundleUseCase) Execute(ctx context.Context, inputPath, outputPath string, config Config) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	// Load input file
	data, err := uc.fileLoader.Load(ctx, inputPath)
	if err != nil {
		return fmt.Errorf("failed to load input file: %w", err)
	}

	if config.MaxFileSize > 0 && int64(len(data)) > config.MaxFileSize {
		return fmt.Errorf("file size %d exceeds maximum allowed size %d", len(data), config.MaxFileSize)
	}

	// Parse as yaml.Node to preserve order
	p := parser.NewParser()
	root, err := p.ParseFile(data)
	if err != nil {
		return fmt.Errorf("failed to parse input file: %w", err)
	}

	// Set output format based on output file extension
	outputFormat := domain.DetectFormat(outputPath)
	p.SetOutputFormat(outputFormat)

	// Get base path
	basePath := getBasePath(inputPath)

	// Resolve all references
	r := resolver.NewResolver(uc.fileLoader)
	domainConfig := domain.Config{
		MaxFileSize: config.MaxFileSize,
		MaxDepth:    config.MaxDepth,
		Inline:      config.Inline,
	}
	if err := r.ResolveNode(ctx, root, basePath, domainConfig); err != nil {
		return fmt.Errorf("failed to resolve references: %w", err)
	}

	// Marshal result
	outputData, err := p.MarshalNode(root)
	if err != nil {
		return fmt.Errorf("failed to marshal result: %w", err)
	}

	// Write output
	if err := uc.fileWriter.Write(outputPath, outputData); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	// Validate if requested
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

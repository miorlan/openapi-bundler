package bundler

import (
	"context"
	"time"

	"github.com/miorlan/openapi-bundler/internal/infrastructure"
	"github.com/miorlan/openapi-bundler/internal/usecase"
)

// Option represents a configuration option for the bundler
type Option func(*Config)

// Config holds the configuration for the bundler
type Config struct {
	Validate    bool
	MaxFileSize int64
	MaxDepth    int
	HTTPTimeout time.Duration
}

// WithValidation enables OpenAPI validation after bundling
func WithValidation(validate bool) Option {
	return func(c *Config) {
		c.Validate = validate
	}
}

// WithMaxFileSize sets the maximum file size in bytes (0 = unlimited)
func WithMaxFileSize(size int64) Option {
	return func(c *Config) {
		c.MaxFileSize = size
	}
}

// WithMaxDepth sets the maximum recursion depth for resolving references (0 = unlimited)
func WithMaxDepth(depth int) Option {
	return func(c *Config) {
		c.MaxDepth = depth
	}
}

// WithHTTPTimeout sets the timeout for HTTP requests
func WithHTTPTimeout(timeout time.Duration) Option {
	return func(c *Config) {
		c.HTTPTimeout = timeout
	}
}

// defaultConfig returns the default configuration
func defaultConfig() *Config {
	return &Config{
		Validate:    false,
		MaxFileSize: 0, // unlimited
		MaxDepth:    0, // unlimited
		HTTPTimeout: 30 * time.Second,
	}
}

// Bundler provides a simple API for bundling OpenAPI specifications
type Bundler struct {
	useCase *usecase.BundleUseCase
	config  *Config
}

// New creates a new Bundler instance with default configuration
func New(opts ...Option) *Bundler {
	config := defaultConfig()
	for _, opt := range opts {
		opt(config)
	}

	fileLoader := infrastructure.NewFileLoaderWithTimeout(config.HTTPTimeout)
	fileWriter := infrastructure.NewFileWriter()
	parser := infrastructure.NewParser()
	referenceResolver := infrastructure.NewReferenceResolver(fileLoader, parser)
	validator := infrastructure.NewValidator()

	useCase := usecase.NewBundleUseCase(
		fileLoader,
		fileWriter,
		parser,
		referenceResolver,
		validator,
	)

	return &Bundler{
		useCase: useCase,
		config:  config,
	}
}

// Bundle bundles an OpenAPI specification from inputPath to outputPath
//
// Example:
//
//	b := bundler.New()
//	err := b.Bundle(context.Background(), "input.yaml", "output.yaml")
func (b *Bundler) Bundle(ctx context.Context, inputPath, outputPath string) error {
	return b.useCase.Execute(ctx, inputPath, outputPath, usecase.Config{
		Validate:    b.config.Validate,
		MaxFileSize: b.config.MaxFileSize,
		MaxDepth:    b.config.MaxDepth,
	})
}

// BundleWithValidation bundles and validates an OpenAPI specification
//
// Example:
//
//	b := bundler.New(bundler.WithValidation(true))
//	err := b.BundleWithValidation(context.Background(), "input.yaml", "output.yaml")
func (b *Bundler) BundleWithValidation(ctx context.Context, inputPath, outputPath string) error {
	return b.useCase.Execute(ctx, inputPath, outputPath, usecase.Config{
		Validate:    true,
		MaxFileSize: b.config.MaxFileSize,
		MaxDepth:    b.config.MaxDepth,
	})
}

// Bundle is a convenience function that creates a new Bundler and bundles the specification
//
// Example:
//
//	err := bundler.Bundle(context.Background(), "input.yaml", "output.yaml")
func Bundle(ctx context.Context, inputPath, outputPath string) error {
	b := New()
	return b.Bundle(ctx, inputPath, outputPath)
}

// BundleWithValidation is a convenience function that bundles and validates
//
// Example:
//
//	err := bundler.BundleWithValidation(context.Background(), "input.yaml", "output.yaml")
func BundleWithValidation(ctx context.Context, inputPath, outputPath string) error {
	b := New(WithValidation(true))
	return b.BundleWithValidation(ctx, inputPath, outputPath)
}


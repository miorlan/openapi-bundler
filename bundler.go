package bundler

import (
	"context"
	"time"

	"github.com/miorlan/openapi-bundler/internal/infrastructure"
	"github.com/miorlan/openapi-bundler/internal/usecase"
)

type Option func(*Config)

type Config struct {
	Validate    bool
	MaxFileSize int64
	MaxDepth    int
	HTTPTimeout time.Duration
}

func WithValidation(validate bool) Option {
	return func(c *Config) {
		c.Validate = validate
	}
}

func WithMaxFileSize(size int64) Option {
	return func(c *Config) {
		c.MaxFileSize = size
	}
}

func WithMaxDepth(depth int) Option {
	return func(c *Config) {
		c.MaxDepth = depth
	}
}

func WithHTTPTimeout(timeout time.Duration) Option {
	return func(c *Config) {
		c.HTTPTimeout = timeout
	}
}

func defaultConfig() *Config {
	return &Config{
		Validate:    false,
		MaxFileSize: 0, // unlimited
		MaxDepth:    0, // unlimited
		HTTPTimeout: 30 * time.Second,
	}
}

type Bundler struct {
	useCase *usecase.BundleUseCase
	config  *Config
}

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

func (b *Bundler) Bundle(ctx context.Context, inputPath, outputPath string) error {
	return b.useCase.Execute(ctx, inputPath, outputPath, usecase.Config{
		Validate:    b.config.Validate,
		MaxFileSize: b.config.MaxFileSize,
		MaxDepth:    b.config.MaxDepth,
	})
}

func (b *Bundler) BundleWithValidation(ctx context.Context, inputPath, outputPath string) error {
	return b.useCase.Execute(ctx, inputPath, outputPath, usecase.Config{
		Validate:    true,
		MaxFileSize: b.config.MaxFileSize,
		MaxDepth:    b.config.MaxDepth,
	})
}

func Bundle(ctx context.Context, inputPath, outputPath string) error {
	b := New()
	return b.Bundle(ctx, inputPath, outputPath)
}

func BundleWithValidation(ctx context.Context, inputPath, outputPath string) error {
	b := New(WithValidation(true))
	return b.BundleWithValidation(ctx, inputPath, outputPath)
}


package main

import (
	"github.com/miorlan/openapi-bundler/internal/infrastructure/loader"
	"github.com/miorlan/openapi-bundler/internal/infrastructure/parser"
	"github.com/miorlan/openapi-bundler/internal/infrastructure/resolver"
	"github.com/miorlan/openapi-bundler/internal/infrastructure/validator"
	"github.com/miorlan/openapi-bundler/internal/infrastructure/writer"
	"github.com/miorlan/openapi-bundler/internal/usecase"
)

func newBundler() *usecase.BundleUseCase {
	fileLoader := loader.NewFileLoader()
	fileWriter := writer.NewFileWriter()
	p := parser.NewParser()
	referenceResolver := resolver.NewReferenceResolver(fileLoader, p)
	v := validator.NewValidator()

	return usecase.NewBundleUseCase(
		fileLoader,
		fileWriter,
		p,
		referenceResolver,
		v,
	)
}


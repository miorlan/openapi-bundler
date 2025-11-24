package main

import (
	"github.com/miorlan/openapi-bundler/internal/infrastructure"
	"github.com/miorlan/openapi-bundler/internal/usecase"
)

func newBundler() *usecase.BundleUseCase {
	fileLoader := infrastructure.NewFileLoader()
	fileWriter := infrastructure.NewFileWriter()
	parser := infrastructure.NewParser()
	referenceResolver := infrastructure.NewReferenceResolver(fileLoader, parser)
	validator := infrastructure.NewValidator()

	return usecase.NewBundleUseCase(
		fileLoader,
		fileWriter,
		parser,
		referenceResolver,
		validator,
	)
}


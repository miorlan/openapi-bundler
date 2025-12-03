package main

import (
	"github.com/miorlan/openapi-bundler/internal/infrastructure/loader"
	"github.com/miorlan/openapi-bundler/internal/infrastructure/validator"
	"github.com/miorlan/openapi-bundler/internal/infrastructure/writer"
	"github.com/miorlan/openapi-bundler/internal/usecase"
)

func newBundler() *usecase.BundleUseCase {
	fileLoader := loader.NewFileLoader()
	fileWriter := writer.NewFileWriter()
	v := validator.NewValidator()

	return usecase.NewBundleUseCase(
		fileLoader,
		fileWriter,
		v,
	)
}

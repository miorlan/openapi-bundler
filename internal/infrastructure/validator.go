package infrastructure

import (
	"fmt"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/miorlan/openapi-bundler/internal/domain"
)

// Validator реализует валидацию OpenAPI спецификаций
type Validator struct{}

// NewValidator создает новый валидатор
func NewValidator() domain.Validator {
	return &Validator{}
}

// Validate валидирует OpenAPI спецификацию
func (v *Validator) Validate(filePath string) error {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = false

	_, err := loader.LoadFromFile(filePath)
	if err != nil {
		return fmt.Errorf("invalid OpenAPI specification: %w", err)
	}

	return nil
}


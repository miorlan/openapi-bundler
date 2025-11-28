package validator

import (
	"fmt"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/miorlan/openapi-bundler/internal/domain"
)

type Validator struct{}

func NewValidator() domain.Validator {
	return &Validator{}
}

func (v *Validator) Validate(filePath string) error {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = false

	_, err := loader.LoadFromFile(filePath)
	if err != nil {
		return fmt.Errorf("invalid OpenAPI specification: %w", err)
	}

	return nil
}


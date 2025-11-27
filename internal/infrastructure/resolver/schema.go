package resolver

import (
	"context"
	"fmt"

	"github.com/miorlan/openapi-bundler/internal/domain"
)

func (r *ReferenceResolver) processSchemaProperties(ctx context.Context, n map[string]interface{}, baseDir string, config domain.Config, depth int) error {
	if properties, ok := n["properties"].(map[string]interface{}); ok {
		for propName, propValue := range properties {
			if propMap, ok := propValue.(map[string]interface{}); ok {
				if err := r.replaceExternalRefsWithContext(ctx, propMap, baseDir, config, depth, false, false); err != nil {
					return fmt.Errorf("failed to process property %s: %w", propName, err)
				}
			}
		}
	}

	if items, ok := n["items"].(map[string]interface{}); ok {
		if err := r.replaceExternalRefsWithContext(ctx, items, baseDir, config, depth, false, false); err != nil {
			return fmt.Errorf("failed to process items: %w", err)
		}
	}

	if additionalProps, ok := n["additionalProperties"].(map[string]interface{}); ok {
		if err := r.replaceExternalRefsWithContext(ctx, additionalProps, baseDir, config, depth, false, false); err != nil {
			return fmt.Errorf("failed to process additionalProperties: %w", err)
		}
	}

	if patternProps, ok := n["patternProperties"].(map[string]interface{}); ok {
		for pattern, patternValue := range patternProps {
			if patternMap, ok := patternValue.(map[string]interface{}); ok {
				if err := r.replaceExternalRefsWithContext(ctx, patternMap, baseDir, config, depth, false, false); err != nil {
					return fmt.Errorf("failed to process patternProperty %s: %w", pattern, err)
				}
			}
		}
	}

	return nil
}


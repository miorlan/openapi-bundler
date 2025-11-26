package infrastructure

import (
	"context"
	"fmt"

	"github.com/miorlan/openapi-bundler/internal/domain"
)

func (r *ReferenceResolver) processCompositionKeywords(ctx context.Context, n map[string]interface{}, baseDir string, config domain.Config, depth int) error {
	compositionKeywords := []string{"allOf", "oneOf", "anyOf"}

	for _, keyword := range compositionKeywords {
		if array, ok := n[keyword].([]interface{}); ok {
			for i, item := range array {
				if itemMap, ok := item.(map[string]interface{}); ok {
					// Пока просто прокидываем стандартный контекст флагами (false, false)
					if err := r.replaceExternalRefsWithContext(ctx, itemMap, baseDir, config, depth, false, false); err != nil {
						return fmt.Errorf("failed to process %s item %d: %w", keyword, i, err)
					}
				}
			}
		}
	}
	
	return nil
}


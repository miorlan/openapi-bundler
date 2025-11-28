package resolver

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/miorlan/openapi-bundler/internal/domain"
)

func (r *ReferenceResolver) expandSectionRefs(ctx context.Context, data map[string]interface{}, baseDir string, config domain.Config) error {
	if err := r.expandPathsSection(ctx, data, baseDir, config); err != nil {
		return err
	}
	return r.expandComponentsSections(ctx, data, baseDir, config)
}

func (r *ReferenceResolver) extractRefFromValue(val interface{}) string {
	if section, ok := val.(map[string]interface{}); ok {
		if refVal, hasRef := section["$ref"]; hasRef {
			if ref, ok := refVal.(string); ok && !strings.HasPrefix(ref, "#") {
				return ref
			}
		}
	} else if ref, ok := val.(string); ok && !strings.HasPrefix(ref, "#") {
		return ref
	}
	return ""
}

func (r *ReferenceResolver) getSectionBaseDir(refPath string) string {
	if strings.HasPrefix(refPath, "http://") || strings.HasPrefix(refPath, "https://") {
		return refPath[:strings.LastIndex(refPath, "/")+1]
	}
	return filepath.Dir(refPath)
}

func (r *ReferenceResolver) expandPathsSection(ctx context.Context, data map[string]interface{}, baseDir string, config domain.Config) error {
	pathsVal, ok := data["paths"]
	if !ok {
		return nil
	}

	refStr := r.extractRefFromValue(pathsVal)
	if refStr == "" {
		return nil
	}

	refPath := r.getRefPath(refStr, baseDir)
	if refPath == "" {
		return fmt.Errorf("invalid paths reference: %s", refStr)
	}

	content, err := r.loadAndParseFile(ctx, refStr, baseDir, config)
	if err != nil {
		return fmt.Errorf("failed to expand paths section: %w", err)
	}

	pathsMap := r.extractSection(content, "paths")
	if pathsMap == nil {
		return nil
	}

	sectionBaseDir := r.getSectionBaseDir(refPath)
	r.pathsBaseDir = sectionBaseDir
	if err := r.replaceExternalRefs(ctx, pathsMap, sectionBaseDir, config, 0); err != nil {
		return fmt.Errorf("failed to process references in paths section: %w", err)
	}

	data["paths"] = pathsMap
	return nil
}

func (r *ReferenceResolver) expandComponentsSections(ctx context.Context, data map[string]interface{}, baseDir string, config domain.Config) error {
	components, ok := data["components"].(map[string]interface{})
	if !ok {
		return nil
	}

	for _, ct := range componentTypes {
		sectionVal, exists := components[ct]
		if !exists {
			continue
		}

		refStr := r.extractRefFromValue(sectionVal)
		if refStr == "" {
			continue
		}

		refPath := r.getRefPath(refStr, baseDir)
		if refPath == "" {
			return fmt.Errorf("invalid components.%s reference: %s", ct, refStr)
		}

		content, err := r.loadAndParseFile(ctx, refStr, baseDir, config)
		if err != nil {
			return fmt.Errorf("failed to expand components.%s section: %w", ct, err)
		}

		sectionMap := r.extractSection(content, "components", ct)
		if sectionMap == nil {
			if m, ok := content.(map[string]interface{}); ok {
				sectionMap = m
			}
		}

		if sectionMap == nil {
			continue
		}

		sectionBaseDir := r.getSectionBaseDir(refPath)
		r.componentsBaseDir[ct] = sectionBaseDir
		
		if ct == "schemas" {
			for schemaName, schemaValue := range sectionMap {
				if schemaMap, ok := schemaValue.(map[string]interface{}); ok {
					if refVal, hasRef := schemaMap["$ref"]; hasRef {
						if refStr, ok := refVal.(string); ok {
							resolvedRef := r.getRefPath(refStr, sectionBaseDir)
							if resolvedRef != "" {
								var normalizedPath string
								if absPath, err := filepath.Abs(resolvedRef); err == nil {
									normalizedPath = absPath
								} else {
									normalizedPath = resolvedRef
								}
								normalizedName := r.normalizeComponentName(schemaName)
								r.schemaFileToName[normalizedPath] = normalizedName
								pathWithoutExt := strings.TrimSuffix(normalizedPath, filepath.Ext(normalizedPath))
								r.schemaFileToName[pathWithoutExt] = normalizedName
							}
						}
					}
				}
			}
		}
		
		components[ct] = sectionMap
		
		if ct == "schemas" {
			r.processingComponentsIndex = true
			for schemaName, schemaValue := range sectionMap {
				if schemaMap, ok := schemaValue.(map[string]interface{}); ok {
					r.currentComponentName = r.normalizeComponentName(schemaName)
					if err := r.replaceExternalRefs(ctx, schemaMap, sectionBaseDir, config, 0); err != nil {
						r.processingComponentsIndex = false
						r.currentComponentName = ""
						return fmt.Errorf("failed to process references in component %s: %w", schemaName, err)
					}
				}
			}
			r.processingComponentsIndex = false
			r.currentComponentName = ""
		} else {
			if err := r.replaceExternalRefs(ctx, sectionMap, sectionBaseDir, config, 0); err != nil {
				return fmt.Errorf("failed to process references in components.%s section: %w", ct, err)
			}
		}
	}

	return nil
}


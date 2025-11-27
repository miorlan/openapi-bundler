package infrastructure

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/miorlan/openapi-bundler/internal/domain"
)

func (r *ReferenceResolver) countExternalRefUsage(ctx context.Context, node interface{}, baseDir string, config domain.Config, depth int) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	if config.MaxDepth > 0 && depth > config.MaxDepth {
		return nil
	}

	switch n := node.(type) {
	case map[string]interface{}:
		currentBaseDir := baseDir
		if _, isPaths := n["paths"]; isPaths {
			if r.pathsBaseDir != "" {
				currentBaseDir = r.pathsBaseDir
			}
		}
		if schemasBaseDir, exists := r.componentsBaseDir["schemas"]; exists && schemasBaseDir != "" {
			if strings.Contains(baseDir, "schemas") || baseDir == schemasBaseDir || strings.HasSuffix(baseDir, "schemas") {
				currentBaseDir = schemasBaseDir
			}
		}

		if pathsMap, ok := n["paths"].(map[string]interface{}); ok {
			pathsBaseDir := baseDir
			if r.pathsBaseDir != "" {
				pathsBaseDir = r.pathsBaseDir
			}
			for _, pathValue := range pathsMap {
				if pathMap, ok := pathValue.(map[string]interface{}); ok {
					if err := r.countExternalRefUsage(ctx, pathMap, pathsBaseDir, config, depth); err != nil {
						return err
					}
				}
			}
		}

		if componentsMap, ok := n["components"].(map[string]interface{}); ok {
			for _, ct := range componentTypes {
				if section, ok := componentsMap[ct].(map[string]interface{}); ok {
					componentBaseDir := r.rootBasePath
					if savedBaseDir, exists := r.componentsBaseDir[ct]; exists && savedBaseDir != "" {
						componentBaseDir = savedBaseDir
					}
					for _, component := range section {
						if err := r.countExternalRefUsage(ctx, component, componentBaseDir, config, depth); err != nil {
							return err
						}
					}
				}
			}
		}

		if refVal, ok := n["$ref"]; ok {
			if refStr, ok := refVal.(string); ok && !strings.HasPrefix(refStr, "#") {
				refParts := strings.SplitN(refStr, "#", 2)
				refPath := refParts[0]
				if refPath != "" {
					baseDirForCount := currentBaseDir
					if r.pathsBaseDir != "" && strings.Contains(baseDir, "paths") {
						baseDirForCount = r.pathsBaseDir
					} else if schemasBaseDir, exists := r.componentsBaseDir["schemas"]; exists && schemasBaseDir != "" {
						refPathForCheck := r.getRefPath(refPath, currentBaseDir)
						if refPathForCheck != "" && strings.Contains(refPathForCheck, "schemas") {
							baseDirForCount = schemasBaseDir
						} else if strings.Contains(baseDir, "schemas") {
							baseDirForCount = schemasBaseDir
						}
					}

					refPathForCount := r.getRefPath(refPath, baseDirForCount)
					if refPathForCount != "" {
						normalizedPath := r.normalizeRefPathForCount(refPathForCount)
						r.externalRefUsageCount[normalizedPath]++
					}
				}
			}
		}

		for _, v := range n {
			if err := r.countExternalRefUsage(ctx, v, currentBaseDir, config, depth); err != nil {
				return err
			}
		}

	case []interface{}:
		for _, item := range n {
			if err := r.countExternalRefUsage(ctx, item, baseDir, config, depth); err != nil {
				return err
			}
		}
	}

	return nil
}

func (r *ReferenceResolver) replaceExternalRefs(ctx context.Context, node interface{}, baseDir string, config domain.Config, depth int) error {
	return r.replaceExternalRefsWithContext(ctx, node, baseDir, config, depth, false, false)
}

func (r *ReferenceResolver) resolveAndReplaceExternalRef(ctx context.Context, ref string, baseDir string, config domain.Config, depth int) (string, error) {
	return r.resolveAndReplaceExternalRefWithContext(ctx, ref, baseDir, config, depth, false)
}

func (r *ReferenceResolver) resolveAndReplaceExternalRefWithContext(ctx context.Context, ref string, baseDir string, config domain.Config, depth int, skipExtraction bool) (string, error) {
	return r.resolveAndReplaceExternalRefWithType(ctx, ref, baseDir, config, depth, "", skipExtraction)
}

func (r *ReferenceResolver) loadAndParseRefFile(ctx context.Context, refPath string, config domain.Config) (interface{}, error) {
	if !strings.HasPrefix(refPath, "http://") && !strings.HasPrefix(refPath, "https://") {
		refPath = filepath.Clean(refPath)
	}

	if cached, ok := r.fileCache[refPath]; ok {
		return cached, nil
	}

	data, err := r.fileLoader.Load(ctx, refPath)
	if err != nil {
		return nil, err
	}

	format := domain.DetectFormat(refPath)
	var parsed interface{}
	if err := r.parser.Unmarshal(data, &parsed, format); err != nil {
		return nil, fmt.Errorf("failed to parse file %s: %w", refPath, err)
	}

	r.fileCache[refPath] = parsed
	return parsed, nil
}


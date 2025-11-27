package infrastructure

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/miorlan/openapi-bundler/internal/domain"
)

func (r *ReferenceResolver) collectExternalRefs(ctx context.Context, node interface{}, baseDir string, config domain.Config, depth int, refs map[string]string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	if config.MaxDepth > 0 && depth >= config.MaxDepth {
		return nil
	}

	switch n := node.(type) {
	case map[string]interface{}:
		if refVal, ok := n["$ref"]; ok {
			if refStr, ok := refVal.(string); ok && !strings.HasPrefix(refStr, "#") {
				refParts := strings.SplitN(refStr, "#", 2)
				refPath := refParts[0]
				if refPath != "" {
					resolvedPath := r.getRefPath(refPath, baseDir)
					if resolvedPath != "" {
						if !strings.HasPrefix(resolvedPath, "http://") && !strings.HasPrefix(resolvedPath, "https://") {
							resolvedPath = filepath.Clean(resolvedPath)
						}
						visitedKey := resolvedPath
						if len(refParts) > 1 {
							visitedKey += "#" + refParts[1]
						}
						if !r.visited[visitedKey] {
							refs[visitedKey] = resolvedPath
						}
					}
				}
			}
		}

		if _, hasAllOf := n["allOf"]; hasAllOf {
			if allOf, ok := n["allOf"].([]interface{}); ok {
				for _, item := range allOf {
					if itemMap, ok := item.(map[string]interface{}); ok {
						if err := r.collectExternalRefs(ctx, itemMap, baseDir, config, depth, refs); err != nil {
							return err
						}
					}
				}
			}
		}
		if _, hasOneOf := n["oneOf"]; hasOneOf {
			if oneOf, ok := n["oneOf"].([]interface{}); ok {
				for _, item := range oneOf {
					if itemMap, ok := item.(map[string]interface{}); ok {
						if err := r.collectExternalRefs(ctx, itemMap, baseDir, config, depth, refs); err != nil {
							return err
						}
					}
				}
			}
		}
		if _, hasAnyOf := n["anyOf"]; hasAnyOf {
			if anyOf, ok := n["anyOf"].([]interface{}); ok {
				for _, item := range anyOf {
					if itemMap, ok := item.(map[string]interface{}); ok {
						if err := r.collectExternalRefs(ctx, itemMap, baseDir, config, depth, refs); err != nil {
							return err
						}
					}
				}
			}
		}

		if _, hasProperties := n["properties"]; hasProperties {
			if properties, ok := n["properties"].(map[string]interface{}); ok {
				for _, propValue := range properties {
					if propMap, ok := propValue.(map[string]interface{}); ok {
						if err := r.collectExternalRefs(ctx, propMap, baseDir, config, depth, refs); err != nil {
							return err
						}
					}
				}
			}
		}

		if _, hasItems := n["items"]; hasItems {
			if items, ok := n["items"].(map[string]interface{}); ok {
				if err := r.collectExternalRefs(ctx, items, baseDir, config, depth, refs); err != nil {
					return err
				}
			}
		}

		if _, hasAdditionalProperties := n["additionalProperties"]; hasAdditionalProperties {
			if additionalProps, ok := n["additionalProperties"].(map[string]interface{}); ok {
				if err := r.collectExternalRefs(ctx, additionalProps, baseDir, config, depth, refs); err != nil {
					return err
				}
			}
		}

		if _, hasPatternProperties := n["patternProperties"]; hasPatternProperties {
			if patternProps, ok := n["patternProperties"].(map[string]interface{}); ok {
				for _, patternValue := range patternProps {
					if patternMap, ok := patternValue.(map[string]interface{}); ok {
						if err := r.collectExternalRefs(ctx, patternMap, baseDir, config, depth, refs); err != nil {
							return err
						}
					}
				}
			}
		}

		if _, hasParameters := n["parameters"]; hasParameters {
			if params, ok := n["parameters"].([]interface{}); ok {
				for _, param := range params {
					if paramMap, ok := param.(map[string]interface{}); ok {
						if err := r.collectExternalRefs(ctx, paramMap, baseDir, config, depth, refs); err != nil {
							return err
						}
					}
				}
			}
		}

		for k, v := range n {
			if k == "$ref" {
				continue
			}
			if err := r.collectExternalRefs(ctx, v, baseDir, config, depth, refs); err != nil {
				return err
			}
		}

	case []interface{}:
		for _, item := range n {
			if err := r.collectExternalRefs(ctx, item, baseDir, config, depth, refs); err != nil {
				return err
			}
		}
	}

	return nil
}

func (r *ReferenceResolver) preloadExternalFiles(ctx context.Context, data map[string]interface{}, basePath string, config domain.Config) error {
	refs := make(map[string]string)
	visitedPaths := make(map[string]bool)

	var collectRecursive func(node interface{}, baseDir string, depth int) error
	collectRecursive = func(node interface{}, baseDir string, depth int) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		if config.MaxDepth > 0 && depth > config.MaxDepth {
			return nil
		}

		switch n := node.(type) {
		case map[string]interface{}:
			if refVal, ok := n["$ref"]; ok {
				if refStr, ok := refVal.(string); ok && !strings.HasPrefix(refStr, "#") {
					refParts := strings.SplitN(refStr, "#", 2)
					refPath := refParts[0]
					if refPath != "" {
						resolvedPath := r.getRefPath(refPath, baseDir)
						if resolvedPath != "" {
							if !strings.HasPrefix(resolvedPath, "http://") && !strings.HasPrefix(resolvedPath, "https://") {
								resolvedPath = filepath.Clean(resolvedPath)
							}
							if !visitedPaths[resolvedPath] {
								visitedPaths[resolvedPath] = true
								refs[resolvedPath] = resolvedPath
								
								if cached, ok := r.fileCache[resolvedPath]; ok {
									if cachedMap, ok := cached.(map[string]interface{}); ok {
										var nextBaseDir string
										if strings.HasPrefix(resolvedPath, "http://") || strings.HasPrefix(resolvedPath, "https://") {
											nextBaseDir = resolvedPath[:strings.LastIndex(resolvedPath, "/")+1]
										} else {
											nextBaseDir = filepath.Dir(resolvedPath)
										}
										if err := collectRecursive(cachedMap, nextBaseDir, depth+1); err != nil {
											return err
										}
									}
								}
							}
						}
					}
				}
			}

			if _, hasAllOf := n["allOf"]; hasAllOf {
				if allOf, ok := n["allOf"].([]interface{}); ok {
					for _, item := range allOf {
						if itemMap, ok := item.(map[string]interface{}); ok {
							if err := collectRecursive(itemMap, baseDir, depth); err != nil {
								return err
							}
						}
					}
				}
			}
			if _, hasOneOf := n["oneOf"]; hasOneOf {
				if oneOf, ok := n["oneOf"].([]interface{}); ok {
					for _, item := range oneOf {
						if itemMap, ok := item.(map[string]interface{}); ok {
							if err := collectRecursive(itemMap, baseDir, depth); err != nil {
								return err
							}
						}
					}
				}
			}
			if _, hasAnyOf := n["anyOf"]; hasAnyOf {
				if anyOf, ok := n["anyOf"].([]interface{}); ok {
					for _, item := range anyOf {
						if itemMap, ok := item.(map[string]interface{}); ok {
							if err := collectRecursive(itemMap, baseDir, depth); err != nil {
								return err
							}
						}
					}
				}
			}

			if _, hasProperties := n["properties"]; hasProperties {
				if properties, ok := n["properties"].(map[string]interface{}); ok {
					for _, propValue := range properties {
						if propMap, ok := propValue.(map[string]interface{}); ok {
							if err := collectRecursive(propMap, baseDir, depth); err != nil {
								return err
							}
						}
					}
				}
			}

			if _, hasItems := n["items"]; hasItems {
				if items, ok := n["items"].(map[string]interface{}); ok {
					if err := collectRecursive(items, baseDir, depth); err != nil {
						return err
					}
				}
			}

			if _, hasAdditionalProperties := n["additionalProperties"]; hasAdditionalProperties {
				if additionalProps, ok := n["additionalProperties"].(map[string]interface{}); ok {
					if err := collectRecursive(additionalProps, baseDir, depth); err != nil {
						return err
					}
				}
			}

			if _, hasPatternProperties := n["patternProperties"]; hasPatternProperties {
				if patternProps, ok := n["patternProperties"].(map[string]interface{}); ok {
					for _, patternValue := range patternProps {
						if patternMap, ok := patternValue.(map[string]interface{}); ok {
							if err := collectRecursive(patternMap, baseDir, depth); err != nil {
								return err
							}
						}
					}
				}
			}

			if _, hasParameters := n["parameters"]; hasParameters {
				if params, ok := n["parameters"].([]interface{}); ok {
					for _, param := range params {
						if paramMap, ok := param.(map[string]interface{}); ok {
							if err := collectRecursive(paramMap, baseDir, depth); err != nil {
								return err
							}
						}
					}
				}
			}

			for k, v := range n {
				if k == "$ref" {
					continue
				}
				if err := collectRecursive(v, baseDir, depth); err != nil {
					return err
				}
			}

		case []interface{}:
			for _, item := range n {
				if err := collectRecursive(item, baseDir, depth); err != nil {
					return err
				}
			}
		}

		return nil
	}

	if err := collectRecursive(data, basePath, 0); err != nil {
		return fmt.Errorf("failed to collect external refs: %w", err)
	}

	if len(refs) == 0 {
		return nil
	}

	paths := make([]string, 0, len(refs))
	for path := range refs {
		if _, exists := r.fileCache[path]; !exists {
			paths = append(paths, path)
		}
	}

	if len(paths) == 0 {
		return nil
	}

	loader, ok := r.fileLoader.(*FileLoader)
	if !ok {
		return nil
	}

	loadedFiles, err := loader.LoadMany(ctx, paths)
	if err != nil {
		return fmt.Errorf("failed to load files in parallel: %w", err)
	}

	for path, data := range loadedFiles {
		if config.MaxFileSize > 0 && int64(len(data)) > config.MaxFileSize {
			return fmt.Errorf("file size %d exceeds maximum allowed size %d", len(data), config.MaxFileSize)
		}

		format := domain.DetectFormat(path)
		var content interface{}
		if err := r.parser.Unmarshal(data, &content, format); err != nil {
			return fmt.Errorf("failed to parse file %s: %w", path, err)
		}
		r.fileCache[path] = content
	}

	for {
		for path := range loadedFiles {
			if content, ok := r.fileCache[path]; ok {
				var nextBaseDir string
				if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
					nextBaseDir = path[:strings.LastIndex(path, "/")+1]
				} else {
					nextBaseDir = filepath.Dir(path)
				}
				if contentMap, ok := content.(map[string]interface{}); ok {
					if err := collectRecursive(contentMap, nextBaseDir, 0); err != nil {
						return err
					}
				}
			}
		}

		newPaths := make([]string, 0)
		for path := range refs {
			if _, exists := r.fileCache[path]; !exists {
				if !visitedPaths[path] {
					visitedPaths[path] = true
					newPaths = append(newPaths, path)
				}
			}
		}

		if len(newPaths) == 0 {
			break
		}

		newLoadedFiles, err := loader.LoadMany(ctx, newPaths)
		if err != nil {
			return fmt.Errorf("failed to load additional files in parallel: %w", err)
		}

		for path, data := range newLoadedFiles {
			if config.MaxFileSize > 0 && int64(len(data)) > config.MaxFileSize {
				return fmt.Errorf("file size %d exceeds maximum allowed size %d", len(data), config.MaxFileSize)
			}

			format := domain.DetectFormat(path)
			var content interface{}
			if err := r.parser.Unmarshal(data, &content, format); err != nil {
				return fmt.Errorf("failed to parse file %s: %w", path, err)
			}
			r.fileCache[path] = content
		}

		loadedFiles = newLoadedFiles
	}

	return nil
}


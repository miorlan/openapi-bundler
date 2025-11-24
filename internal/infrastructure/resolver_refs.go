package infrastructure

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/miorlan/openapi-bundler/internal/domain"
)

func (r *ReferenceResolver) replaceExternalRefs(ctx context.Context, node interface{}, baseDir string, config domain.Config, depth int) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	if config.MaxDepth > 0 && depth >= config.MaxDepth {
		return fmt.Errorf("maximum recursion depth %d exceeded", config.MaxDepth)
	}

	switch n := node.(type) {
	case map[string]interface{}:
		if _, hasAllOf := n["allOf"]; hasAllOf {
			if allOf, ok := n["allOf"].([]interface{}); ok {
				for i, item := range allOf {
					if itemMap, ok := item.(map[string]interface{}); ok {
						if err := r.replaceExternalRefs(ctx, itemMap, baseDir, config, depth); err != nil {
							return fmt.Errorf("failed to process allOf item %d: %w", i, err)
						}
					}
				}
			}
		}
		if _, hasOneOf := n["oneOf"]; hasOneOf {
			if oneOf, ok := n["oneOf"].([]interface{}); ok {
				for i, item := range oneOf {
					if itemMap, ok := item.(map[string]interface{}); ok {
						if err := r.replaceExternalRefs(ctx, itemMap, baseDir, config, depth); err != nil {
							return fmt.Errorf("failed to process oneOf item %d: %w", i, err)
						}
					}
				}
			}
		}
		if _, hasAnyOf := n["anyOf"]; hasAnyOf {
			if anyOf, ok := n["anyOf"].([]interface{}); ok {
				for i, item := range anyOf {
					if itemMap, ok := item.(map[string]interface{}); ok {
						if err := r.replaceExternalRefs(ctx, itemMap, baseDir, config, depth); err != nil {
							return fmt.Errorf("failed to process anyOf item %d: %w", i, err)
						}
					}
				}
			}
		}

		if _, hasProperties := n["properties"]; hasProperties {
			if properties, ok := n["properties"].(map[string]interface{}); ok {
				for propName, propValue := range properties {
					if propMap, ok := propValue.(map[string]interface{}); ok {
						if err := r.replaceExternalRefs(ctx, propMap, baseDir, config, depth); err != nil {
							return fmt.Errorf("failed to process property %s: %w", propName, err)
						}
					}
				}
			}
		}

		if _, hasItems := n["items"]; hasItems {
			if items, ok := n["items"].(map[string]interface{}); ok {
				if err := r.replaceExternalRefs(ctx, items, baseDir, config, depth); err != nil {
					return fmt.Errorf("failed to process items: %w", err)
				}
			}
		}

		if _, hasAdditionalProperties := n["additionalProperties"]; hasAdditionalProperties {
			if additionalProps, ok := n["additionalProperties"].(map[string]interface{}); ok {
				if err := r.replaceExternalRefs(ctx, additionalProps, baseDir, config, depth); err != nil {
					return fmt.Errorf("failed to process additionalProperties: %w", err)
				}
			}
		}

		if _, hasPatternProperties := n["patternProperties"]; hasPatternProperties {
			if patternProps, ok := n["patternProperties"].(map[string]interface{}); ok {
				for pattern, patternValue := range patternProps {
					if patternMap, ok := patternValue.(map[string]interface{}); ok {
						if err := r.replaceExternalRefs(ctx, patternMap, baseDir, config, depth); err != nil {
							return fmt.Errorf("failed to process patternProperty %s: %w", pattern, err)
						}
					}
				}
			}
		}

		if refVal, ok := n["$ref"]; ok {
			refStr, ok := refVal.(string)
			if !ok {
				return &domain.ErrInvalidReference{Ref: fmt.Sprintf("%v", refVal)}
			}

			if strings.HasPrefix(refStr, "#") {
				return nil
			}

			refParts := strings.SplitN(refStr, "#", 2)
			refPath := refParts[0]
			fragment := ""
			if len(refParts) > 1 {
				fragment = "#" + refParts[1]
			}

			if refPath == "" {
				return nil
			}

			if fragment != "" && strings.HasPrefix(fragment, "#/components/") {
				internalRef, err := r.resolveAndReplaceExternalRef(ctx, refStr, baseDir, config, depth)
				if err != nil {
					return fmt.Errorf("failed to replace external ref %s: %w", refStr, err)
				}

				if internalRef != "" {
					n["$ref"] = internalRef
				}

				return nil
			}

			internalRef, err := r.resolveAndReplaceExternalRef(ctx, refStr, baseDir, config, depth)
			if err == nil && internalRef != "" {
				n["$ref"] = internalRef
				return nil
			}

			if err != nil {
				expanded, expandErr := r.expandPathRef(ctx, refStr, baseDir, config, depth)
				if expandErr != nil {
					return fmt.Errorf("failed to replace external ref %s: %w", refStr, err)
				}

				if expanded != nil {
					for k, v := range expanded {
						n[k] = v
					}
					delete(n, "$ref")
				}
			}

			return nil
		}

		for k, v := range n {
			if k == "paths" {
				if pathsMap, ok := v.(map[string]interface{}); ok {
					pathsBaseDir := baseDir
					if r.pathsBaseDir != "" {
						pathsBaseDir = r.pathsBaseDir
					}
					for pathKey, pathValue := range pathsMap {
						if pathMap, ok := pathValue.(map[string]interface{}); ok {
							if err := r.replaceExternalRefs(ctx, pathMap, pathsBaseDir, config, depth); err != nil {
								return fmt.Errorf("failed to process path %s: %w", pathKey, err)
							}
						}
					}
					continue
				}
			}

			if k == "components" {
				if componentsMap, ok := v.(map[string]interface{}); ok {
					for _, ct := range componentTypes {
						if section, ok := componentsMap[ct].(map[string]interface{}); ok {
							componentBaseDir := baseDir
							if savedBaseDir, exists := r.componentsBaseDir[ct]; exists && savedBaseDir != "" {
								componentBaseDir = savedBaseDir
							}
							for name, component := range section {
								if componentStr, ok := component.(string); ok {
									if !strings.HasPrefix(componentStr, "#") {
										internalRef, err := r.resolveAndReplaceExternalRef(ctx, componentStr, componentBaseDir, config, depth)
										if err == nil && internalRef != "" {
											section[name] = map[string]interface{}{
												"$ref": internalRef,
											}
											continue
										}
										return fmt.Errorf("failed to resolve component %s/%s: %s: %w", ct, name, componentStr, err)
									}
								}
								if componentMap, ok := component.(map[string]interface{}); ok {
									if err := r.replaceExternalRefs(ctx, componentMap, componentBaseDir, config, depth); err != nil {
										return fmt.Errorf("failed to process component %s/%s: %w", ct, name, err)
									}
								}
							}
						}
					}
					continue
				}
			}

			if err := r.replaceExternalRefs(ctx, v, baseDir, config, depth); err != nil {
				return fmt.Errorf("failed to process field %s: %w", k, err)
			}
		}

	case []interface{}:
		for i, item := range n {
			if err := r.replaceExternalRefs(ctx, item, baseDir, config, depth); err != nil {
				return fmt.Errorf("failed to process array item %d: %w", i, err)
			}
		}
	case string:
		return nil
	}

	return nil
}

func (r *ReferenceResolver) resolveAndReplaceExternalRef(ctx context.Context, ref string, baseDir string, config domain.Config, depth int) (string, error) {
	refParts := strings.SplitN(ref, "#", 2)
	refPath := refParts[0]
	fragment := ""
	if len(refParts) > 1 {
		fragment = "#" + refParts[1]
	}

	if refPath == "" {
		return "", nil
	}

	refPath = r.getRefPath(refPath, baseDir)
	if refPath == "" {
		return "", &domain.ErrInvalidReference{Ref: ref}
	}

	if !strings.HasPrefix(refPath, "http://") && !strings.HasPrefix(refPath, "https://") {
		refPath = filepath.Clean(refPath)
	}

	visitedKey := refPath + fragment
	if r.visited[visitedKey] {
		if internalRef, ok := r.componentRefs[visitedKey]; ok {
			return internalRef, nil
		}
		return "", &domain.ErrCircularReference{Path: visitedKey}
	}
	r.visited[visitedKey] = true
	defer delete(r.visited, visitedKey)

	var content interface{}
	if cached, ok := r.fileCache[refPath]; ok {
		content = cached
	} else {
		data, err := r.fileLoader.Load(ctx, refPath)
		if err != nil {
			if os.IsNotExist(err) {
				return "", &ErrFileNotFound{Path: refPath}
			}
			return "", fmt.Errorf("failed to load file: %w", err)
		}

		if config.MaxFileSize > 0 && int64(len(data)) > config.MaxFileSize {
			return "", fmt.Errorf("file size %d exceeds maximum allowed size %d", len(data), config.MaxFileSize)
		}

		format := domain.DetectFormat(refPath)
		if err := r.parser.Unmarshal(data, &content, format); err != nil {
			return "", fmt.Errorf("failed to parse file: %w", err)
		}
		r.fileCache[refPath] = content
	}

	var nextBaseDir string
	if strings.HasPrefix(refPath, "http://") || strings.HasPrefix(refPath, "https://") {
		nextBaseDir = refPath[:strings.LastIndex(refPath, "/")+1]
	} else {
		nextBaseDir = filepath.Dir(refPath)
	}

	var componentContent interface{}
	componentType := "schemas"
	
	if fragment != "" {
		extracted, err := r.resolveJSONPointer(content, fragment)
		if err != nil {
			return "", fmt.Errorf("failed to resolve fragment %s: %w", fragment, err)
		}
		componentContent = extracted
		
		parts := strings.Split(strings.TrimPrefix(fragment, "#/"), "/")
		if len(parts) >= 2 && parts[0] == "components" {
			componentType = parts[1]
		}
	} else {
		if contentMap, ok := content.(map[string]interface{}); ok {
			if comps, ok := contentMap["components"].(map[string]interface{}); ok {
				totalComponents := 0
				var foundComponent interface{}
				var foundType string
				for _, ct := range componentTypes {
					if section, ok := comps[ct].(map[string]interface{}); ok {
						totalComponents += len(section)
						if len(section) == 1 {
							for _, comp := range section {
								foundComponent = comp
								foundType = ct
								break
							}
						}
					}
				}
				if totalComponents == 1 && foundComponent != nil {
					componentContent = foundComponent
					componentType = foundType
				} else if totalComponents > 0 {
					return "", fmt.Errorf("external file contains multiple components, specify fragment: %s", ref)
				} else {
					return "", fmt.Errorf("external file does not contain components: %s", ref)
				}
			} else {
				if _, hasType := contentMap["type"]; hasType {
					componentContent = contentMap
					componentType = "schemas"
				} else {
					return "", fmt.Errorf("external file does not contain components section or schema: %s", ref)
				}
			}
		} else {
			return "", fmt.Errorf("external file content is not a map: %s", ref)
		}
	}

	preferredName := r.getPreferredComponentName(ref, fragment, componentType)
	components := r.rootDoc["components"].(map[string]interface{})
	section, ok := components[componentType].(map[string]interface{})
	if !ok {
		section = make(map[string]interface{})
		components[componentType] = section
	}

	var componentName string
	if existingComponent, exists := section[preferredName]; exists {
		if existingComponentMap, ok := existingComponent.(map[string]interface{}); ok {
			if existingRef, hasRef := existingComponentMap["$ref"]; hasRef {
				if existingRefStr, ok := existingRef.(string); ok && existingRefStr == ref {
					componentName = preferredName
				}
			}
		}
		if componentName == "" {
			componentName = r.ensureUniqueComponentName(preferredName, section, componentType)
		}
	} else {
		componentName = preferredName
	}

	if componentContent == nil {
		return "", fmt.Errorf("component content is nil for ref: %s", ref)
	}

	if _, ok := componentContent.(map[string]interface{}); !ok {
		if _, ok := componentContent.([]interface{}); !ok {
			return "", fmt.Errorf("component content must be an object or array, got %T for ref: %s", componentContent, ref)
		}
	}

	if existingRef, ok := r.componentRefs[visitedKey]; ok {
		return existingRef, nil
	}

	componentCopy := r.deepCopy(componentContent)
	if err := r.replaceExternalRefs(ctx, componentCopy, nextBaseDir, config, depth+1); err != nil {
		return "", fmt.Errorf("failed to process component: %w", err)
	}

	componentHash := r.hashComponent(componentCopy)
	if existingName, exists := r.componentHashes[componentHash]; exists {
		if existingComponent, ok := r.components[componentType][existingName]; ok {
			if r.componentsEqual(existingComponent, componentCopy) {
				internalRef := "#/components/" + componentType + "/" + existingName
				r.componentRefs[visitedKey] = internalRef
				return internalRef, nil
			}
		}
	}

	if _, exists := section[componentName]; !exists {
		r.components[componentType][componentName] = componentCopy
		r.componentHashes[componentHash] = componentName
	} else {
		if _, existsInCollected := r.components[componentType][componentName]; !existsInCollected {
			r.components[componentType][componentName] = componentCopy
			r.componentHashes[componentHash] = componentName
		}
	}

	internalRef := "#/components/" + componentType + "/" + componentName
	r.componentRefs[visitedKey] = internalRef

	return internalRef, nil
}

func (r *ReferenceResolver) expandPathRef(ctx context.Context, ref string, baseDir string, config domain.Config, depth int) (map[string]interface{}, error) {
	refPath := r.getRefPath(ref, baseDir)
	if refPath == "" {
		return nil, &domain.ErrInvalidReference{Ref: ref}
	}

	if !strings.HasPrefix(refPath, "http://") && !strings.HasPrefix(refPath, "https://") {
		refPath = filepath.Clean(refPath)
	}

	visitedKey := refPath
	if r.visited[visitedKey] {
		return nil, &domain.ErrCircularReference{Path: visitedKey}
	}
	r.visited[visitedKey] = true
	defer delete(r.visited, visitedKey)

	var content interface{}
	if cached, ok := r.fileCache[refPath]; ok {
		content = cached
	} else {
		data, err := r.fileLoader.Load(ctx, refPath)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, &ErrFileNotFound{Path: refPath}
			}
			return nil, fmt.Errorf("failed to load file: %w", err)
		}

		if config.MaxFileSize > 0 && int64(len(data)) > config.MaxFileSize {
			return nil, fmt.Errorf("file size %d exceeds maximum allowed size %d", len(data), config.MaxFileSize)
		}

		format := domain.DetectFormat(refPath)
		if err := r.parser.Unmarshal(data, &content, format); err != nil {
			return nil, fmt.Errorf("failed to parse file: %w", err)
		}
		r.fileCache[refPath] = content
	}

	var nextBaseDir string
	if strings.HasPrefix(refPath, "http://") || strings.HasPrefix(refPath, "https://") {
		nextBaseDir = refPath[:strings.LastIndex(refPath, "/")+1]
	} else {
		nextBaseDir = filepath.Dir(refPath)
	}

	contentMap, ok := content.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("path reference content must be a map, got %T", content)
	}

	expanded := r.deepCopy(contentMap).(map[string]interface{})
	if err := r.replaceExternalRefs(ctx, expanded, nextBaseDir, config, depth+1); err != nil {
		return nil, fmt.Errorf("failed to process references in path: %w", err)
	}

	return expanded, nil
}


package resolver

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/miorlan/openapi-bundler/internal/domain"
	"github.com/miorlan/openapi-bundler/internal/infrastructure/errors"
)

func (r *ReferenceResolver) replaceExternalRefsWithContext(ctx context.Context, node interface{}, baseDir string, config domain.Config, depth int, inContentContext bool, inSchemaContext bool) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	if config.MaxDepth > 0 && depth > config.MaxDepth {
		return fmt.Errorf("maximum recursion depth %d exceeded", config.MaxDepth)
	}

	switch n := node.(type) {
	case map[string]interface{}:
		if paramsArray, ok := n["parameters"].([]interface{}); ok {
			for i, param := range paramsArray {
				if paramMap, ok := param.(map[string]interface{}); ok {
					if refVal, hasRef := paramMap["$ref"]; hasRef {
						if refStr, ok := refVal.(string); ok && !strings.HasPrefix(refStr, "#") {
							internalRef, err := r.resolveAndReplaceExternalRefWithType(ctx, refStr, baseDir, config, depth, "parameters", false)
							if err == nil && internalRef != "" {
								paramsArray[i] = map[string]interface{}{
									"$ref": internalRef,
								}
								continue
							}
						} else if refStr, ok := refVal.(string); ok && strings.HasPrefix(refStr, "#/components/parameters/") {
							continue
						}
					} else {
						// Process schema refs inside inline parameters (don't extract to components)
						if schemaVal, hasSchema := paramMap["schema"]; hasSchema {
							if schemaMap, ok := schemaVal.(map[string]interface{}); ok {
								if err := r.replaceExternalRefsWithContext(ctx, schemaMap, baseDir, config, depth, inContentContext, true); err != nil {
									return fmt.Errorf("failed to process schema in parameter: %w", err)
								}
							}
						}
						// Keep inline parameters as-is (like swagger-cli does)
					}
				}
			}
		}

		if _, hasAllOf := n["allOf"]; hasAllOf {
			if allOf, ok := n["allOf"].([]interface{}); ok {
				for i, item := range allOf {
					if itemMap, ok := item.(map[string]interface{}); ok {
						if err := r.replaceExternalRefsWithContext(ctx, itemMap, baseDir, config, depth, inContentContext, false); err != nil {
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
						if err := r.replaceExternalRefsWithContext(ctx, itemMap, baseDir, config, depth, inContentContext, false); err != nil {
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
						if err := r.replaceExternalRefsWithContext(ctx, itemMap, baseDir, config, depth, inContentContext, false); err != nil {
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
						if err := r.replaceExternalRefsWithContext(ctx, propMap, baseDir, config, depth, inContentContext, false); err != nil {
							return fmt.Errorf("failed to process property %s: %w", propName, err)
						}
					}
				}
			}
		}

		if _, hasItems := n["items"]; hasItems {
			if items, ok := n["items"].(map[string]interface{}); ok {
				if err := r.replaceExternalRefsWithContext(ctx, items, baseDir, config, depth, inContentContext, false); err != nil {
					return fmt.Errorf("failed to process items: %w", err)
				}
			}
		}

		if _, hasAdditionalProperties := n["additionalProperties"]; hasAdditionalProperties {
			if additionalProps, ok := n["additionalProperties"].(map[string]interface{}); ok {
				if err := r.replaceExternalRefsWithContext(ctx, additionalProps, baseDir, config, depth, inContentContext, false); err != nil {
					return fmt.Errorf("failed to process additionalProperties: %w", err)
				}
			}
		}

		if _, hasPatternProperties := n["patternProperties"]; hasPatternProperties {
			if patternProps, ok := n["patternProperties"].(map[string]interface{}); ok {
				for pattern, patternValue := range patternProps {
					if patternMap, ok := patternValue.(map[string]interface{}); ok {
						if err := r.replaceExternalRefsWithContext(ctx, patternMap, baseDir, config, depth, inContentContext, false); err != nil {
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
				internalRef, err := r.resolveAndReplaceExternalRefWithContext(ctx, refStr, baseDir, config, depth, false)
				if err != nil {
					return fmt.Errorf("failed to replace external ref %s: %w", refStr, err)
				}

				if internalRef != "" {
					n["$ref"] = internalRef
				}

				return nil
			}

			// Try to find schema by resolved path
			resolvedPath := r.getRefPath(refPath, baseDir)
			if resolvedPath != "" {
				if schemaName, exists := r.findSchemaByPath(resolvedPath); exists {
					// Skip self-references when processing components
					if !(r.processingComponentsIndex && r.currentComponentName == schemaName) {
						n["$ref"] = "#/components/schemas/" + schemaName
						return nil
					}
				}
			}

			// Check if this is a schema file reference not in _index.yaml - inline it
			if strings.Contains(refPath, "schemas") {
				inlineRefPath := r.getRefPath(refStr, baseDir)
				if inlineRefPath != "" {
					content, loadErr := r.loadAndParseFile(ctx, refStr, baseDir, config)
					if loadErr == nil {
						if contentMap, ok := content.(map[string]interface{}); ok {
							expanded := r.deepCopy(contentMap).(map[string]interface{})
							nextBaseDir := filepath.Dir(inlineRefPath)
							if err := r.replaceExternalRefsWithContext(ctx, expanded, nextBaseDir, config, depth+1, false, true); err != nil {
								return fmt.Errorf("failed to process schema references: %w", err)
							}
							for k, v := range expanded {
								n[k] = v
							}
							delete(n, "$ref")
							return nil
						}
					}
				}
			}

			// Try to expand as path reference
			expandedPath, expandErr := r.expandPathRef(ctx, refStr, baseDir, config, depth)
			if expandErr == nil && expandedPath != nil {
				for k, v := range expandedPath {
					n[k] = v
				}
				delete(n, "$ref")
				return nil
			}

			// Try to inline content for non-schema files
			inlineRefPath := r.getRefPath(refStr, baseDir)
			if inlineRefPath != "" {
				content, loadErr := r.loadAndParseFile(ctx, refStr, baseDir, config)
				if loadErr == nil {
					if contentMap, ok := content.(map[string]interface{}); ok {
						expanded := r.deepCopy(contentMap).(map[string]interface{})
						nextBaseDir := filepath.Dir(inlineRefPath)
						if err := r.replaceExternalRefsWithContext(ctx, expanded, nextBaseDir, config, depth+1, false, false); err != nil {
							return fmt.Errorf("failed to process references: %w", err)
						}
						for k, v := range expanded {
							n[k] = v
						}
						delete(n, "$ref")
						return nil
					}
				}
			}

			// Fallback: try to resolve as component
			componentRef, err := r.resolveAndReplaceExternalRefWithType(ctx, refStr, baseDir, config, depth, "schemas", false)
			if err != nil {
				return fmt.Errorf("failed to resolve external ref %s: %w", refStr, err)
			}

			if componentRef != "" {
				n["$ref"] = componentRef
				return nil
			}

			return fmt.Errorf("failed to resolve external ref %s: no component created", refStr)
		}

		for k, v := range n {
			if k == "content" {
				if contentMap, ok := v.(map[string]interface{}); ok {
					if err := r.replaceExternalRefsWithContext(ctx, contentMap, baseDir, config, depth, false, false); err != nil {
						return fmt.Errorf("failed to process content: %w", err)
					}
					continue
				}
			}

			if k == "parameters" {
				continue
			}

			if k == "paths" {
				if pathsMap, ok := v.(map[string]interface{}); ok {
					pathsBaseDir := baseDir
					if r.pathsBaseDir != "" {
						pathsBaseDir = r.pathsBaseDir
					}
					for pathKey, pathValue := range pathsMap {
						if pathMap, ok := pathValue.(map[string]interface{}); ok {
							if refVal, hasRef := pathMap["$ref"]; hasRef {
								if refStr, ok := refVal.(string); ok && !strings.HasPrefix(refStr, "#") {
									expanded, expandErr := r.expandPathRef(ctx, refStr, pathsBaseDir, config, depth)
									if expandErr == nil && expanded != nil {
										for k, v := range expanded {
											pathMap[k] = v
										}
										delete(pathMap, "$ref")
									}
								}
							}
							if err := r.replaceExternalRefsWithContext(ctx, pathMap, pathsBaseDir, config, depth, false, false); err != nil {
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
							componentBaseDir := r.rootBasePath
							if savedBaseDir, exists := r.componentsBaseDir[ct]; exists && savedBaseDir != "" {
								componentBaseDir = savedBaseDir
							}
							for name, component := range section {
								if component == nil {
									delete(section, name)
									continue
								}
								if componentStr, ok := component.(string); ok {
									if !strings.HasPrefix(componentStr, "#") {
										internalRef, err := r.resolveAndReplaceExternalRefWithContext(ctx, componentStr, componentBaseDir, config, depth, false)
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
									if err := r.replaceExternalRefsWithContext(ctx, componentMap, componentBaseDir, config, depth, false, false); err != nil {
										return fmt.Errorf("failed to process component %s/%s: %w", ct, name, err)
									}
								}
							}
						}
					}
					continue
				}
			}

			if err := r.replaceExternalRefsWithContext(ctx, v, baseDir, config, depth, inContentContext, false); err != nil {
				return fmt.Errorf("failed to process field %s: %w", k, err)
			}
		}

	case []interface{}:
		for i, item := range n {
			if itemMap, ok := item.(map[string]interface{}); ok {
				if refVal, hasRef := itemMap["$ref"]; hasRef {
					if refStr, ok := refVal.(string); ok && !strings.HasPrefix(refStr, "#") {
						internalRef, err := r.resolveAndReplaceExternalRefWithContext(ctx, refStr, baseDir, config, depth, false)
						if err == nil && internalRef != "" {
							n[i] = map[string]interface{}{
								"$ref": internalRef,
							}
							continue
						}
					} else if refStr, ok := refVal.(string); ok && strings.HasPrefix(refStr, "#/components/parameters/") {
						continue
					}
				}
				if len(itemMap) == 1 {
					if _, hasRef := itemMap["$ref"]; hasRef {
						continue
					}
				}
			}
			if err := r.replaceExternalRefsWithContext(ctx, item, baseDir, config, depth, false, false); err != nil {
				return fmt.Errorf("failed to process array item %d: %w", i, err)
			}
		}
	case string:
		return nil
	}

	return nil
}

func (r *ReferenceResolver) resolveAndReplaceExternalRefWithType(ctx context.Context, ref string, baseDir string, config domain.Config, depth int, preferredComponentType string, skipExtraction bool) (string, error) {
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
			if skipExtraction {
				return "", nil
			}
			return internalRef, nil
		}
		return "", &domain.ErrCircularReference{Path: visitedKey}
	}

	if !skipExtraction {
		r.visited[visitedKey] = true
		defer delete(r.visited, visitedKey)
	}

	content, err := r.loadAndParseRefFile(ctx, refPath, config)
	if err != nil {
		return "", err
	}

	var nextBaseDir string
	if strings.HasPrefix(refPath, "http://") || strings.HasPrefix(refPath, "https://") {
		nextBaseDir = refPath[:strings.LastIndex(refPath, "/")+1]
	} else {
		nextBaseDir = filepath.Dir(refPath)
	}

	var componentContent interface{}
	componentType := preferredComponentType
	if componentType == "" {
		componentType = "schemas"
	}

	if fragment != "" {
		extracted, err := r.resolveJSONPointer(content, fragment)
		if err != nil {
			return "", fmt.Errorf("failed to resolve fragment %s: %w", fragment, err)
		}
		componentContent = extracted

		parts := strings.Split(strings.TrimPrefix(fragment, "#/"), "/")
		if len(parts) >= 2 && parts[0] == "components" {
			componentType = parts[1]

			if contentMap, ok := content.(map[string]interface{}); ok {
				if comps, ok := contentMap["components"].(map[string]interface{}); ok {
					for _, ct := range componentTypes {
						if section, ok := comps[ct].(map[string]interface{}); ok {
							for name := range section {
								normalizedName := r.normalizeComponentName(name)
								preRegKey := refPath + "#/components/" + ct + "/" + name
								if _, exists := r.componentRefs[preRegKey]; !exists {
									r.componentRefs[preRegKey] = "#/components/" + ct + "/" + normalizedName
								}
							}
						}
					}

					for _, ct := range componentTypes {
						if section, ok := comps[ct].(map[string]interface{}); ok {
							components := r.rootDoc["components"].(map[string]interface{})
							targetSection, ok := components[ct].(map[string]interface{})
							if !ok {
								targetSection = make(map[string]interface{})
								components[ct] = targetSection
							}

							for name, comp := range section {
								normalizedName := r.normalizeComponentName(name)
								if _, exists := targetSection[normalizedName]; !exists {
									compCopy := r.deepCopy(comp)
									if compCopy != nil {
										if err := r.replaceExternalRefsWithContext(ctx, compCopy, nextBaseDir, config, depth+1, false, false); err != nil {
											return "", fmt.Errorf("failed to process component %s: %w", name, err)
										}
										targetSection[normalizedName] = compCopy
										r.components[ct][normalizedName] = compCopy
										componentHash := r.hashComponent(compCopy)
										r.componentHashes[componentHash] = normalizedName
									}
								}
							}
						}
					}
				}
			}
		}
	} else {
		if contentMap, ok := content.(map[string]interface{}); ok {
			if comps, ok := contentMap["components"].(map[string]interface{}); ok {
				totalComponents := 0
				var foundComponent interface{}
				var foundComponentName string
				var foundType string
				for _, ct := range componentTypes {
					if section, ok := comps[ct].(map[string]interface{}); ok {
						totalComponents += len(section)
						if len(section) == 1 {
							for name, comp := range section {
								foundComponent = comp
								foundComponentName = name
								foundType = ct
								break
							}
						}
					}
				}
				if totalComponents == 1 && foundComponent != nil {
					componentContent = foundComponent
					componentType = foundType
					if foundComponentName != "" {
						originalRef := ref
						if r.refToComponentName[originalRef] == "" {
							r.refToComponentName[originalRef] = r.normalizeComponentName(foundComponentName)
						}
					}
				} else if totalComponents > 0 {
					return "", fmt.Errorf("external file contains multiple components, specify fragment: %s", ref)
				} else {
					return "", fmt.Errorf("external file does not contain components: %s", ref)
				}
			} else {
				if _, hasGet := contentMap["get"]; hasGet {
					return "", fmt.Errorf("external file contains path definition, not a component: %s", ref)
				}
				if _, hasPost := contentMap["post"]; hasPost {
					return "", fmt.Errorf("external file contains path definition, not a component: %s", ref)
				}
				if _, hasPut := contentMap["put"]; hasPut {
					return "", fmt.Errorf("external file contains path definition, not a component: %s", ref)
				}
				if _, hasDelete := contentMap["delete"]; hasDelete {
					return "", fmt.Errorf("external file contains path definition, not a component: %s", ref)
				}
				if _, hasPatch := contentMap["patch"]; hasPatch {
					return "", fmt.Errorf("external file contains path definition, not a component: %s", ref)
				}
				if preferredComponentType != "" {
					componentContent = contentMap
					componentType = preferredComponentType
				} else if _, hasType := contentMap["type"]; hasType {
					componentContent = contentMap
					componentType = "schemas"
				} else if _, hasIn := contentMap["in"]; hasIn {
					componentContent = contentMap
					componentType = "parameters"
				} else {
					return "", fmt.Errorf("external file does not contain components section or schema: %s", ref)
				}
			}
		} else {
			return "", fmt.Errorf("external file content is not a map: %s", ref)
		}
	}

	originalRef := ref
	if fragment != "" {
		originalRef = ref + fragment
	}

	var componentName string
	var foundComponentName string

	if fragment == "" {
		if contentMap, ok := content.(map[string]interface{}); ok {
			if comps, ok := contentMap["components"].(map[string]interface{}); ok {
				for _, ct := range componentTypes {
					if section, ok := comps[ct].(map[string]interface{}); ok {
						if len(section) == 1 {
							for name := range section {
								foundComponentName = name
								break
							}
						}
					}
				}
			}
		}
	}

	if cachedName, exists := r.refToComponentName[originalRef]; exists && cachedName != "" {
		componentName = cachedName
	} else if foundComponentName != "" {
		componentName = r.normalizeComponentName(foundComponentName)
		r.refToComponentName[originalRef] = componentName
	} else if fragment != "" {
		parts := strings.Split(strings.TrimPrefix(fragment, "#/"), "/")
		if len(parts) >= 3 && parts[0] == "components" && parts[1] == componentType {
			componentName = r.normalizeComponentName(parts[2])
			r.refToComponentName[originalRef] = componentName
		} else {
			componentName = r.getPreferredComponentName(ref, fragment, componentType, componentContent)
			r.refToComponentName[originalRef] = componentName
		}
	} else {
		componentName = r.getPreferredComponentName(ref, fragment, componentType, componentContent)
		r.refToComponentName[originalRef] = componentName
	}

	if skipExtraction {
		if componentContent != nil {
			if componentMap, ok := componentContent.(map[string]interface{}); ok {
				if err := r.replaceExternalRefsWithContext(ctx, componentMap, nextBaseDir, config, depth+1, false, false); err != nil {
					return "", fmt.Errorf("failed to process component content: %w", err)
				}
			}
		}
		return "", nil
	}

	components := r.rootDoc["components"].(map[string]interface{})
	section, ok := components[componentType].(map[string]interface{})
	if !ok {
		section = make(map[string]interface{})
		components[componentType] = section
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
	if componentCopy == nil {
		return "", fmt.Errorf("component copy is nil for ref: %s", ref)
	}

	internalRefPreview := "#/components/" + componentType + "/" + componentName
	r.componentRefs[visitedKey] = internalRefPreview

	if err := r.replaceExternalRefsWithContext(ctx, componentCopy, nextBaseDir, config, depth+1, false, false); err != nil {
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
		if existingComponent, ok := section[existingName]; ok {
			if existingMap, ok := existingComponent.(map[string]interface{}); ok {
				if _, hasRef := existingMap["$ref"]; hasRef {
					if len(existingMap) == 1 {
						section[existingName] = componentCopy
						r.components[componentType][existingName] = componentCopy
						r.componentHashes[componentHash] = existingName
						internalRef := "#/components/" + componentType + "/" + existingName
						r.componentRefs[visitedKey] = internalRef
						return internalRef, nil
					}
				}
			}
			if r.componentsEqual(existingComponent, componentCopy) {
				internalRef := "#/components/" + componentType + "/" + existingName
				r.componentRefs[visitedKey] = internalRef
				return internalRef, nil
			}
		}
	}

	if fragment != "" {
		parts := strings.Split(strings.TrimPrefix(fragment, "#/"), "/")
		if len(parts) >= 3 && parts[0] == "components" && parts[1] == componentType {
			originalName := parts[2]
			normalizedOriginalName := r.normalizeComponentName(originalName)
			if normalizedOriginalName == componentName {
				if componentMap, ok := componentCopy.(map[string]interface{}); ok {
					if refVal, hasRef := componentMap["$ref"]; hasRef {
						if refStr, ok := refVal.(string); ok {
							if strings.HasPrefix(refStr, "#/components/"+componentType+"/") {
								refName := strings.TrimPrefix(refStr, "#/components/"+componentType+"/")
								if refName == componentName {
									return "", &domain.ErrCircularReference{Path: visitedKey + " -> self-reference to " + componentName}
								}
							}
						}
					}
				}
			}
		}
	}

	if existingComponent, exists := section[componentName]; exists {
		if existingMap, ok := existingComponent.(map[string]interface{}); ok {
			if refVal, hasRef := existingMap["$ref"]; hasRef {
				if len(existingMap) == 1 {
					if refStr, ok := refVal.(string); ok {
						expectedRef := "#/components/" + componentType + "/" + componentName
						if refStr == expectedRef {
							section[componentName] = componentCopy
							for name, comp := range r.components[componentType] {
								if r.componentsEqual(comp, componentCopy) && name != componentName {
									delete(r.components[componentType], name)
									break
								}
							}
							r.components[componentType][componentName] = componentCopy
							r.componentHashes[componentHash] = componentName
							internalRef := "#/components/" + componentType + "/" + componentName
							r.componentRefs[visitedKey] = internalRef
							return internalRef, nil
						}
					}
					section[componentName] = componentCopy
					for name, comp := range r.components[componentType] {
						if r.componentsEqual(comp, componentCopy) && name != componentName {
							delete(r.components[componentType], name)
							break
						}
					}
					r.components[componentType][componentName] = componentCopy
					r.componentHashes[componentHash] = componentName
					internalRef := "#/components/" + componentType + "/" + componentName
					r.componentRefs[visitedKey] = internalRef
					return internalRef, nil
				}
			}
		}

		if r.componentsEqual(existingComponent, componentCopy) {
			internalRef := "#/components/" + componentType + "/" + componentName
			r.componentRefs[visitedKey] = internalRef
			return internalRef, nil
		}

		componentName = r.ensureUniqueComponentName(componentName, section, componentType)

		if componentName != r.refToComponentName[originalRef] {
			r.refToComponentName[originalRef] = componentName
		}
	} else {
		componentName = r.ensureUniqueComponentName(componentName, section, componentType)

		if componentName != r.refToComponentName[originalRef] {
			r.refToComponentName[originalRef] = componentName
		}
	}

	if !skipExtraction {
		if _, existsInCollected := r.components[componentType][componentName]; !existsInCollected {
			r.components[componentType][componentName] = componentCopy
			r.componentHashes[componentHash] = componentName
		}

		internalRef := "#/components/" + componentType + "/" + componentName
		r.componentRefs[visitedKey] = internalRef

		return internalRef, nil
	}

	return "", nil
}

func (r *ReferenceResolver) expandPathRef(ctx context.Context, ref string, baseDir string, config domain.Config, depth int) (map[string]interface{}, error) {
	refPath := r.getRefPath(ref, baseDir)
	if refPath == "" {
		return nil, &domain.ErrInvalidReference{Ref: ref}
	}

	if !strings.HasPrefix(refPath, "http://") && !strings.HasPrefix(refPath, "https://") {
		refPath = filepath.Clean(refPath)
	}

	// Don't inline schema files - they should be resolved as components
	// Check if this is a schema file by checking the path
	if strings.Contains(refPath, "schemas") || strings.Contains(strings.ToLower(refPath), "schema") {
		// Check if this schema file is already registered
		var normalizedPath string
		if absPath, err := filepath.Abs(refPath); err == nil {
			normalizedPath = absPath
		} else {
			normalizedPath = refPath
		}

		// Check if schema is registered
		if schemaName, exists := r.schemaFileToName[normalizedPath]; exists {
			return nil, fmt.Errorf("schema file should be resolved as component: %s -> #/components/schemas/%s", refPath, schemaName)
		}

		pathWithoutExt := strings.TrimSuffix(normalizedPath, filepath.Ext(normalizedPath))
		if schemaName, exists := r.schemaFileToName[pathWithoutExt]; exists {
			return nil, fmt.Errorf("schema file should be resolved as component: %s -> #/components/schemas/%s", refPath, schemaName)
		}

		// If file is in schemas directory but not registered, don't inline it
		// Let it be processed as a component reference instead
		if strings.Contains(filepath.Dir(refPath), "schemas") {
			return nil, fmt.Errorf("schema file should not be inlined: %s", refPath)
		}
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
				return nil, &errors.ErrFileNotFound{Path: refPath}
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
	if err := r.replaceExternalRefsWithContext(ctx, expanded, nextBaseDir, config, depth+1, false, false); err != nil {
		return nil, fmt.Errorf("failed to process references in path: %w", err)
	}

	return expanded, nil
}

func (r *ReferenceResolver) liftComponentRefs(ctx context.Context, data map[string]interface{}, config domain.Config) error {
	components, ok := data["components"].(map[string]interface{})
	if !ok {
		return nil
	}

	var checkCycle func(currentType string, section map[string]interface{}, startName string, visited map[string]bool) bool
	checkCycle = func(currentType string, section map[string]interface{}, startName string, visited map[string]bool) bool {
		key := currentType + "/" + startName
		if visited[key] {
			return true
		}
		visited[key] = true
		defer delete(visited, key)

		component, exists := section[startName]
		if !exists {
			return false
		}

		if componentMap, ok := component.(map[string]interface{}); ok {
			if refVal, hasRef := componentMap["$ref"]; hasRef {
				if len(componentMap) == 1 {
					if refStr, ok := refVal.(string); ok {
						parts := strings.Split(strings.TrimPrefix(refStr, "#/components/"), "/")
						if len(parts) >= 2 {
							refType := parts[0]
							refName := parts[1]
							if refSection, ok := components[refType].(map[string]interface{}); ok {
								return checkCycle(refType, refSection, refName, visited)
							}
						}
					}
				}
			}
		}
		return false
	}

	for _, ct := range componentTypes {
		if ct == "schemas" {
			continue
		}

		section, ok := components[ct].(map[string]interface{})
		if !ok {
			continue
		}

		toLift := make(map[string]string)

		for name, component := range section {
			if componentMap, ok := component.(map[string]interface{}); ok {
				if refVal, hasRef := componentMap["$ref"]; hasRef {
					if len(componentMap) == 1 {
						if refStr, ok := refVal.(string); ok {
							if strings.HasPrefix(refStr, "#/components/"+ct+"/") {
								refName := strings.TrimPrefix(refStr, "#/components/"+ct+"/")

								if refName == name {
									continue
								}

								visited := make(map[string]bool)
								if checkCycle(ct, section, refName, visited) {
									continue
								}

								if targetComponent, exists := section[refName]; exists {
									if targetMap, ok := targetComponent.(map[string]interface{}); ok {
										if _, hasTargetRef := targetMap["$ref"]; hasTargetRef {
											if len(targetMap) == 1 {
												continue
											}
										}
									}
									toLift[name] = refName
								}
							}
						}
					}
				}
			}
		}

		for name, refName := range toLift {
			if targetComponent, exists := section[refName]; exists {
				if name == refName {
					continue
				}
				targetCopy := r.deepCopy(targetComponent)
				section[name] = targetCopy
			}
		}
	}

	return nil
}

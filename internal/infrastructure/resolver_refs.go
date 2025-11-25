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
	return r.replaceExternalRefsWithContext(ctx, node, baseDir, config, depth, false, false)
}

func (r *ReferenceResolver) replaceExternalRefsWithContext(ctx context.Context, node interface{}, baseDir string, config domain.Config, depth int, inContentContext bool, inSchemaContext bool) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	if config.MaxDepth > 0 && depth > config.MaxDepth {
		return fmt.Errorf("maximum recursion depth %d exceeded", config.MaxDepth)
	}

	switch n := node.(type) {
	case map[string]interface{}:
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

			// Если мы в контексте schema внутри content, не извлекаем схемы в components
			// Внешние $ref должны быть разрешены и заменены на полное содержимое inline
			if inSchemaContext && inContentContext {
				// Проверяем, является ли это внешним $ref
				if !strings.HasPrefix(refStr, "#") {
					// Внешний $ref - загружаем содержимое и заменяем $ref на полное содержимое
					expanded, err := r.expandExternalRefInline(ctx, refStr, baseDir, config, depth)
					if err == nil && expanded != nil {
						// Заменяем $ref на полное содержимое
						for k, v := range expanded {
							n[k] = v
						}
						delete(n, "$ref")
						// Обрабатываем вложенные ссылки в развернутом содержимом
						if err := r.replaceExternalRefsWithContext(ctx, n, baseDir, config, depth, true, true); err != nil {
							return fmt.Errorf("failed to process expanded schema: %w", err)
						}
						return nil
					}
					// Если не удалось разрешить, продолжаем обычную обработку
				} else {
					// Внутренняя ссылка в schema внутри content - обрабатываем, но не извлекаем
					return nil
				}
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
				// Если мы в контексте schema внутри content, не обрабатываем через обычный путь
				if inSchemaContext && inContentContext {
					return nil
				}
				internalRef, err := r.resolveAndReplaceExternalRefWithContext(ctx, refStr, baseDir, config, depth, false)
				if err != nil {
					return fmt.Errorf("failed to replace external ref %s: %w", refStr, err)
				}

				if internalRef != "" {
					n["$ref"] = internalRef
				}

				return nil
			}

			// Если мы в контексте schema внутри content, не обрабатываем через обычный путь
			// (уже обработано выше через expandExternalRefInline)
			if inSchemaContext && inContentContext {
				return nil
			}

			internalRef, err := r.resolveAndReplaceExternalRefWithContext(ctx, refStr, baseDir, config, depth, false)
			if err == nil && internalRef != "" {
				n["$ref"] = internalRef
				return nil
			}

			if err != nil {
				expanded, expandErr := r.expandPathRef(ctx, refStr, baseDir, config, depth)
				if expandErr == nil && expanded != nil {
					for k, v := range expanded {
						n[k] = v
					}
					delete(n, "$ref")
					return nil
				}
				return fmt.Errorf("failed to replace external ref %s: %w", refStr, err)
			}

			return nil
		}

		for k, v := range n {
			// Специальная обработка для content (responses, requestBody)
			// Inline схемы в content должны оставаться inline, а не извлекаться в components
			if k == "content" {
				if contentMap, ok := v.(map[string]interface{}); ok {
					// Обрабатываем content с флагом, что мы в контексте content
					if err := r.replaceExternalRefsWithContext(ctx, contentMap, baseDir, config, depth, true, false); err != nil {
						return fmt.Errorf("failed to process content: %w", err)
					}
					continue
				}
			}

			// Специальная обработка для schema в content (responses, requestBody)
			// Inline схемы должны оставаться inline, а не извлекаться в components
			if k == "schema" && inContentContext {
				if schemaMap, ok := v.(map[string]interface{}); ok {
					// Проверяем, есть ли внешний $ref в schema
					if refVal, hasRef := schemaMap["$ref"]; hasRef {
						if refStr, ok := refVal.(string); ok {
							// Если это внешний $ref, заменяем на inline содержимое
							if !strings.HasPrefix(refStr, "#") {
								expanded, err := r.expandExternalRefInline(ctx, refStr, baseDir, config, depth)
								if err == nil && expanded != nil {
									// Заменяем $ref на полное содержимое
									for k, v := range expanded {
										schemaMap[k] = v
									}
									delete(schemaMap, "$ref")
									// Обрабатываем вложенные ссылки в развернутом содержимом
									if err := r.replaceExternalRefsWithContext(ctx, schemaMap, baseDir, config, depth, true, true); err != nil {
										return fmt.Errorf("failed to process expanded schema: %w", err)
									}
									continue
								}
							}
						}
					}
					// Обрабатываем schema в content с флагом, что мы в контексте schema
					// Это предотвратит извлечение inline схем в components
					if err := r.replaceExternalRefsWithContext(ctx, schemaMap, baseDir, config, depth, true, true); err != nil {
						return fmt.Errorf("failed to process schema: %w", err)
					}
					continue
				}
			}

			// Специальная обработка для параметров в методах HTTP (get, post, put, delete, etc.)
			// Параметры должны оставаться как $ref, а не разворачиваться
			// ЭТО ДОЛЖНО БЫТЬ ПЕРЕД обработкой paths, чтобы сработать для параметров в методах
			if k == "parameters" {
				if paramsArray, ok := v.([]interface{}); ok {
					for i, param := range paramsArray {
						if paramMap, ok := param.(map[string]interface{}); ok {
							if refVal, hasRef := paramMap["$ref"]; hasRef {
								if refStr, ok := refVal.(string); ok && !strings.HasPrefix(refStr, "#") {
									// Это внешняя ссылка на параметр
									// Создаём компонент в components.parameters, но оставляем $ref в массиве
									internalRef, err := r.resolveAndReplaceExternalRefWithType(ctx, refStr, baseDir, config, depth, "parameters", false)
									if err == nil && internalRef != "" {
										// Заменяем элемент массива на только $ref
										paramsArray[i] = map[string]interface{}{
											"$ref": internalRef,
										}
										continue
									}
								} else if refStr, ok := refVal.(string); ok && strings.HasPrefix(refStr, "#/components/parameters/") {
									// Это уже внутренняя ссылка на параметр - не обрабатываем дальше
									continue
								}
							} else {
								// Параметр уже развернут (не содержит $ref)
								// Проверяем, не совпадает ли он с каким-либо компонентом в components.parameters
								// Если да, заменяем на $ref
								if components, ok := r.rootDoc["components"].(map[string]interface{}); ok {
									if paramsSection, ok := components["parameters"].(map[string]interface{}); ok {
										paramHash := r.hashComponent(paramMap)
										for paramName, paramComponent := range paramsSection {
											if paramCompMap, ok := paramComponent.(map[string]interface{}); ok {
												// Пропускаем компоненты, которые являются только $ref
												if _, hasRef := paramCompMap["$ref"]; hasRef && len(paramCompMap) == 1 {
													continue
												}
												if r.componentsEqual(paramMap, paramCompMap) || r.hashComponent(paramCompMap) == paramHash {
													// Найден совпадающий компонент - заменяем на $ref
													paramsArray[i] = map[string]interface{}{
														"$ref": "#/components/parameters/" + paramName,
													}
													goto nextParam
												}
											}
										}
									}
								}
								// Также проверяем в r.components (еще не добавленных в rootDoc)
								if paramsSection, ok := r.components["parameters"]; ok {
									paramHash := r.hashComponent(paramMap)
									for paramName, paramComponent := range paramsSection {
										if paramCompMap, ok := paramComponent.(map[string]interface{}); ok {
											// Пропускаем компоненты, которые являются только $ref
											if _, hasRef := paramCompMap["$ref"]; hasRef && len(paramCompMap) == 1 {
												continue
											}
											if r.componentsEqual(paramMap, paramCompMap) || r.hashComponent(paramCompMap) == paramHash {
												// Найден совпадающий компонент - заменяем на $ref
												normalizedName := r.normalizeComponentName(paramName)
												paramsArray[i] = map[string]interface{}{
													"$ref": "#/components/parameters/" + normalizedName,
												}
												goto nextParam
											}
										}
									}
								}
							}
							// Если элемент содержит только $ref (внутреннюю), не обрабатываем рекурсивно
							if len(paramMap) == 1 {
								if _, hasRef := paramMap["$ref"]; hasRef {
									continue
								}
							}
						}
					nextParam:
						// Для параметров без внешней ссылки обрабатываем рекурсивно (внутренние ссылки)
						// НО только если параметр не был заменен на $ref
						if paramMap, ok := paramsArray[i].(map[string]interface{}); ok {
							if _, hasRef := paramMap["$ref"]; !hasRef {
								if err := r.replaceExternalRefsWithContext(ctx, paramsArray[i], baseDir, config, depth, false, false); err != nil {
									return fmt.Errorf("failed to process parameter %d: %w", i, err)
								}
								// После обработки проверяем, не совпадает ли параметр с компонентом
								// Если да, заменяем на $ref
								if updatedParamMap, ok := paramsArray[i].(map[string]interface{}); ok {
									if _, hasRef := updatedParamMap["$ref"]; !hasRef {
										// Параметр все еще развернут - проверяем совпадение с компонентами
										paramHash := r.hashComponent(updatedParamMap)
										if components, ok := r.rootDoc["components"].(map[string]interface{}); ok {
											if paramsSection, ok := components["parameters"].(map[string]interface{}); ok {
												for paramName, paramComponent := range paramsSection {
													if paramCompMap, ok := paramComponent.(map[string]interface{}); ok {
														// Пропускаем компоненты, которые являются только $ref
														if _, hasRef := paramCompMap["$ref"]; hasRef && len(paramCompMap) == 1 {
															continue
														}
														if r.componentsEqual(updatedParamMap, paramCompMap) || r.hashComponent(paramCompMap) == paramHash {
															// Найден совпадающий компонент - заменяем на $ref
															paramsArray[i] = map[string]interface{}{
																"$ref": "#/components/parameters/" + paramName,
															}
															break
														}
													}
												}
											}
										}
										// Также проверяем в r.components
										if paramsSection, ok := r.components["parameters"]; ok {
											for paramName, paramComponent := range paramsSection {
												if paramCompMap, ok := paramComponent.(map[string]interface{}); ok {
													// Пропускаем компоненты, которые являются только $ref
													if _, hasRef := paramCompMap["$ref"]; hasRef && len(paramCompMap) == 1 {
														continue
													}
													if r.componentsEqual(updatedParamMap, paramCompMap) || r.hashComponent(paramCompMap) == paramHash {
														// Найден совпадающий компонент - заменяем на $ref
														normalizedName := r.normalizeComponentName(paramName)
														paramsArray[i] = map[string]interface{}{
															"$ref": "#/components/parameters/" + normalizedName,
														}
														break
													}
												}
											}
										}
									}
								}
							}
						}
					}
					continue
				}
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
										// Внешняя ссылка - разрешаем и заменяем на внутреннюю
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
									// Обрабатываем компонент (внутренние $ref будут "подняты" позже в liftComponentRefs)
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
		// Специальная обработка для массивов параметров
		// Параметры должны оставаться как $ref, а не разворачиваться
		for i, item := range n {
			if itemMap, ok := item.(map[string]interface{}); ok {
				// Проверяем, является ли это параметром с внешней ссылкой
				if refVal, hasRef := itemMap["$ref"]; hasRef {
					if refStr, ok := refVal.(string); ok && !strings.HasPrefix(refStr, "#") {
						// Это внешняя ссылка на параметр
						// Создаём компонент в components.parameters, но оставляем $ref в массиве
						internalRef, err := r.resolveAndReplaceExternalRefWithContext(ctx, refStr, baseDir, config, depth, false)
						if err == nil && internalRef != "" {
							// Заменяем элемент массива на только $ref
							n[i] = map[string]interface{}{
								"$ref": internalRef,
							}
							continue
						}
					} else if refStr, ok := refVal.(string); ok && strings.HasPrefix(refStr, "#/components/parameters/") {
						// Это уже внутренняя ссылка на параметр - не обрабатываем дальше
						continue
					}
				}
				// Если элемент содержит только $ref (внутреннюю), не обрабатываем рекурсивно
				if len(itemMap) == 1 {
					if _, hasRef := itemMap["$ref"]; hasRef {
						continue
					}
				}
			}
			// Для остальных элементов массива обрабатываем рекурсивно
			if err := r.replaceExternalRefsWithContext(ctx, item, baseDir, config, depth, false, false); err != nil {
				return fmt.Errorf("failed to process array item %d: %w", i, err)
			}
		}
	case string:
		return nil
	}

	return nil
}

func (r *ReferenceResolver) resolveAndReplaceExternalRef(ctx context.Context, ref string, baseDir string, config domain.Config, depth int) (string, error) {
	return r.resolveAndReplaceExternalRefWithType(ctx, ref, baseDir, config, depth, "", false)
}

func (r *ReferenceResolver) resolveAndReplaceExternalRefWithContext(ctx context.Context, ref string, baseDir string, config domain.Config, depth int, skipExtraction bool) (string, error) {
	return r.resolveAndReplaceExternalRefWithType(ctx, ref, baseDir, config, depth, "", skipExtraction)
}

// loadAndParseRefFile загружает и парсит файл по ссылке (общая логика для resolveAndReplaceExternalRefWithType и expandExternalRefInline)
func (r *ReferenceResolver) loadAndParseRefFile(ctx context.Context, refPath string, config domain.Config) (interface{}, error) {
	if cached, ok := r.fileCache[refPath]; ok {
		return cached, nil
	}

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
	var content interface{}
	if err := r.parser.Unmarshal(data, &content, format); err != nil {
		return nil, fmt.Errorf("failed to parse file: %w", err)
	}

	r.fileCache[refPath] = content
	return content, nil
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
			// Если skipExtraction = true, не возвращаем internalRef, чтобы не создавать компонент
			if skipExtraction {
				return "", nil
			}
			return internalRef, nil
		}
		return "", &domain.ErrCircularReference{Path: visitedKey}
	}
	
	// Если skipExtraction = true, не помечаем как посещенный, чтобы не создавать компонент
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
		}
	} else {
		// Нет фрагмента - пытаемся извлечь компонент из файла
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
							// Если только один компонент, извлекаем его имя
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
					// Используем имя компонента из файла
					if foundComponentName != "" {
						// Сохраняем имя в кэш сразу
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
				// Файл содержит компонент напрямую (не в components)
				// Проверяем тип компонента по содержимому или используем preferredComponentType
				if preferredComponentType != "" {
					// Используем предпочтительный тип (например, "parameters")
					componentContent = contentMap
					componentType = preferredComponentType
				} else if _, hasType := contentMap["type"]; hasType {
					// Это схема
					componentContent = contentMap
					componentType = "schemas"
				} else if _, hasIn := contentMap["in"]; hasIn {
					// Это параметр (имеет поле "in")
					componentContent = contentMap
					componentType = "parameters"
				} else {
					return "", fmt.Errorf("external file does not contain components section or schema: %s", ref)
				}
				// Имя будет определено из имени файла в getPreferredComponentName
			}
		} else {
			return "", fmt.Errorf("external file content is not a map: %s", ref)
		}
	}

	// Проверяем кэш по $ref
	originalRef := ref
	if fragment != "" {
		originalRef = ref + fragment
	}
	
	var componentName string
	var foundComponentName string
	
	// Если компонент был извлечён из файла без фрагмента, используем его имя из файла
	if fragment == "" {
		if contentMap, ok := content.(map[string]interface{}); ok {
			if comps, ok := contentMap["components"].(map[string]interface{}); ok {
				for _, ct := range componentTypes {
					if section, ok := comps[ct].(map[string]interface{}); ok {
						if len(section) == 1 {
							// Извлекаем имя компонента из файла
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
		// Используем имя из кэша
		componentName = cachedName
	} else if foundComponentName != "" {
		// ПРИОРИТЕТ: Используем имя компонента из файла (например, ChangePasswordRequest из файла)
		componentName = r.normalizeComponentName(foundComponentName)
		r.refToComponentName[originalRef] = componentName
	} else if fragment != "" {
		// Если есть фрагмент, используем имя из фрагмента
		parts := strings.Split(strings.TrimPrefix(fragment, "#/"), "/")
		if len(parts) >= 3 && parts[0] == "components" && parts[1] == componentType {
			componentName = r.normalizeComponentName(parts[2])
			r.refToComponentName[originalRef] = componentName
		} else {
			// Вычисляем имя по стратегии
			componentName = r.getPreferredComponentName(ref, fragment, componentType, componentContent)
			r.refToComponentName[originalRef] = componentName
		}
	} else {
		// Вычисляем имя по стратегии (имя файла, title и т.д.)
		componentName = r.getPreferredComponentName(ref, fragment, componentType, componentContent)
		// Сохраняем в кэш
		r.refToComponentName[originalRef] = componentName
	}
	
	// Если skipExtraction = true, не извлекаем компонент в components
	// Это используется для inline схем в content (responses, requestBody)
	if skipExtraction {
		// Возвращаем пустую строку, чтобы не создавать компонент
		// Но все равно обрабатываем вложенные ссылки в компоненте
		if componentContent != nil {
			if componentMap, ok := componentContent.(map[string]interface{}); ok {
				// Обрабатываем вложенные ссылки, но не извлекаем компонент
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

	// Проверяем, не был ли уже обработан этот компонент
	if existingRef, ok := r.componentRefs[visitedKey]; ok {
		return existingRef, nil
	}

	componentCopy := r.deepCopy(componentContent)
	if componentCopy == nil {
		return "", fmt.Errorf("component copy is nil for ref: %s", ref)
	}
	
	// Обрабатываем компонент (разрешаем вложенные ссылки)
	if err := r.replaceExternalRefsWithContext(ctx, componentCopy, nextBaseDir, config, depth+1, false, false); err != nil {
		return "", fmt.Errorf("failed to process component: %w", err)
	}

	// Проверяем дедупликацию по хешу ПЕРЕД выбором имени
	componentHash := r.hashComponent(componentCopy)
	
	// Увеличиваем счетчик использования компонента
	r.componentUsageCount[componentHash]++
	
	// Если компонент с таким же содержимым уже существует, используем его имя
	if existingName, exists := r.componentHashes[componentHash]; exists {
		// Проверяем, что компонент действительно существует
		if existingComponent, ok := r.components[componentType][existingName]; ok {
			if r.componentsEqual(existingComponent, componentCopy) {
				// Компонент уже существует, используем существующую ссылку
				internalRef := "#/components/" + componentType + "/" + existingName
				r.componentRefs[visitedKey] = internalRef
				return internalRef, nil
			}
		}
		// Также проверяем в финальной секции
		if existingComponent, ok := section[existingName]; ok {
			// Если существующий компонент является только $ref, заменяем его
			if existingMap, ok := existingComponent.(map[string]interface{}); ok {
				if _, hasRef := existingMap["$ref"]; hasRef {
					if len(existingMap) == 1 {
						// Существующий компонент - это только $ref, заменяем на реальное содержимое
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

	// Проверяем, не создаём ли мы самоссылку
	if fragment != "" {
		parts := strings.Split(strings.TrimPrefix(fragment, "#/"), "/")
		if len(parts) >= 3 && parts[0] == "components" && parts[1] == componentType {
			originalName := parts[2]
			normalizedOriginalName := r.normalizeComponentName(originalName)
			if normalizedOriginalName == componentName {
				// Проверяем, не ссылается ли компонент сам на себя
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

	// ВАЖНО: Проверяем, существует ли компонент с таким именем в секции ПЕРЕД вызовом ensureUniqueComponentName
	// Это нужно, чтобы заменить компоненты, которые являются только $ref
	if existingComponent, exists := section[componentName]; exists {
		// Компонент уже существует в секции
		// Проверяем, не является ли существующий компонент только $ref
		if existingMap, ok := existingComponent.(map[string]interface{}); ok {
			if refVal, hasRef := existingMap["$ref"]; hasRef {
				if len(existingMap) == 1 {
					// Существующий компонент - это только $ref
					// Проверяем, не ссылается ли он сам на себя
					if refStr, ok := refVal.(string); ok {
						expectedRef := "#/components/" + componentType + "/" + componentName
						if refStr == expectedRef {
							// Это самоссылка (Error: { $ref: '#/components/schemas/Error' })
							// Заменяем на реальное содержимое в секции и в собранных компонентах
							section[componentName] = componentCopy
							// Удаляем компонент из r.components, если он был добавлен с другим именем
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
					// Это ссылка на другой компонент, заменяем на реальное содержимое
					section[componentName] = componentCopy
					// Удаляем компонент из r.components, если он был добавлен с другим именем
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
		
		// Проверяем, не тот ли это же компонент
		if r.componentsEqual(existingComponent, componentCopy) {
			// Это тот же компонент, используем существующее имя
			internalRef := "#/components/" + componentType + "/" + componentName
			r.componentRefs[visitedKey] = internalRef
			return internalRef, nil
		}
		
		// Разные компоненты с одинаковым именем - используем уникальное имя
		// НО только если существующий компонент НЕ является только $ref
		componentName = r.ensureUniqueComponentName(componentName, section, componentType)
		
		// Обновляем кэш, если имя изменилось из-за конфликта
		if componentName != r.refToComponentName[originalRef] {
			r.refToComponentName[originalRef] = componentName
		}
	} else {
		// Компонент не существует в секции, проверяем уникальность имени
		componentName = r.ensureUniqueComponentName(componentName, section, componentType)
		
		// Обновляем кэш, если имя изменилось из-за конфликта
		if componentName != r.refToComponentName[originalRef] {
			r.refToComponentName[originalRef] = componentName
		}
	}

	// Добавляем компонент в r.components только если его еще нет и если skipExtraction = false
	if !skipExtraction {
		if _, existsInCollected := r.components[componentType][componentName]; !existsInCollected {
			r.components[componentType][componentName] = componentCopy
			r.componentHashes[componentHash] = componentName
		}

		internalRef := "#/components/" + componentType + "/" + componentName
		r.componentRefs[visitedKey] = internalRef

		return internalRef, nil
	}

	// Если skipExtraction = true, возвращаем пустую строку, чтобы не создавать компонент
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
	if err := r.replaceExternalRefsWithContext(ctx, expanded, nextBaseDir, config, depth+1, false, false); err != nil {
		return nil, fmt.Errorf("failed to process references in path: %w", err)
	}

	return expanded, nil
}

// expandExternalRefInline загружает содержимое внешнего $ref и возвращает его для inline-замены
// НЕ создает компонент в components
// ВАЖНО: Не использует кэш componentRefs, чтобы избежать создания компонента
func (r *ReferenceResolver) expandExternalRefInline(ctx context.Context, ref string, baseDir string, config domain.Config, depth int) (map[string]interface{}, error) {
	refParts := strings.SplitN(ref, "#", 2)
	refPath := refParts[0]
	fragment := ""
	if len(refParts) > 1 {
		fragment = "#" + refParts[1]
	}

	if refPath == "" {
		return nil, fmt.Errorf("empty ref path")
	}

	refPath = r.getRefPath(refPath, baseDir)
	if refPath == "" {
		return nil, &domain.ErrInvalidReference{Ref: ref}
	}

	if !strings.HasPrefix(refPath, "http://") && !strings.HasPrefix(refPath, "https://") {
		refPath = filepath.Clean(refPath)
	}

	// ВАЖНО: Не проверяем visitedKey и componentRefs, чтобы избежать использования кэша
	// и создания компонента. Мы хотим только загрузить содержимое для inline-замены.
	// Также не помечаем visitedKey как посещенный, чтобы не создавать компонент через другой путь.

	content, err := r.loadAndParseRefFile(ctx, refPath, config)
	if err != nil {
		return nil, err
	}

	var componentContent interface{}
	if fragment != "" {
		extracted, err := r.resolveJSONPointer(content, fragment)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve fragment %s: %w", fragment, err)
		}
		componentContent = extracted
	} else {
		// Нет фрагмента - файл содержит схему напрямую
		if contentMap, ok := content.(map[string]interface{}); ok {
			componentContent = contentMap
		} else {
			return nil, fmt.Errorf("external file content is not a map: %s", ref)
		}
	}

	// Копируем содержимое для inline-замены
	componentCopy := r.deepCopy(componentContent)
	if componentCopy == nil {
		return nil, fmt.Errorf("component copy is nil for ref: %s", ref)
	}

	// Обрабатываем вложенные ссылки, но НЕ извлекаем в components
	nextBaseDir := baseDir
	if strings.HasPrefix(refPath, "http://") || strings.HasPrefix(refPath, "https://") {
		nextBaseDir = refPath[:strings.LastIndex(refPath, "/")+1]
	} else {
		nextBaseDir = filepath.Dir(refPath)
	}

	if err := r.replaceExternalRefsWithContext(ctx, componentCopy, nextBaseDir, config, depth+1, true, true); err != nil {
		return nil, fmt.Errorf("failed to process component content: %w", err)
	}

	if componentMap, ok := componentCopy.(map[string]interface{}); ok {
		return componentMap, nil
	}

	return nil, fmt.Errorf("component content is not a map: %s", ref)
}

// inlineSingleUseComponents заменяет $ref на inline содержимое для компонентов, используемых только один раз
// Исключает компоненты, используемые внутри других схем (в properties, items и т.д.)
func (r *ReferenceResolver) inlineSingleUseComponents(ctx context.Context, data map[string]interface{}) error {
	components, ok := data["components"].(map[string]interface{})
	if !ok {
		return nil
	}

	// Собираем компоненты, которые используются только один раз
	singleUseComponents := make(map[string]string) // componentRef -> componentHash
	for _, ct := range componentTypes {
		section, ok := components[ct].(map[string]interface{})
		if !ok {
			continue
		}
		for name, component := range section {
			componentHash := r.hashComponent(component)
			if r.componentUsageCount[componentHash] == 1 {
				componentRef := "#/components/" + ct + "/" + name
				singleUseComponents[componentRef] = componentHash
			}
		}
	}

	if len(singleUseComponents) == 0 {
		return nil
	}

	// Заменяем $ref на inline содержимое для компонентов, используемых только один раз
	// Исключаем компоненты, используемые внутри других схем (в properties, items и т.д.)
	return r.replaceRefsWithInline(ctx, data, singleUseComponents, components, false)
}

// replaceRefsWithInline заменяет $ref на inline содержимое в документе
// skipNested: если true, пропускает $ref внутри properties, items и т.д.
func (r *ReferenceResolver) replaceRefsWithInline(ctx context.Context, node interface{}, singleUseComponents map[string]string, components map[string]interface{}, skipNested bool) error {
	switch n := node.(type) {
	case map[string]interface{}:
		// Проверяем, не находимся ли мы внутри properties, items и т.д.
		isNested := skipNested || n["properties"] != nil || n["items"] != nil || n["additionalProperties"] != nil
		
		if refVal, ok := n["$ref"]; ok {
			if refStr, ok := refVal.(string); ok {
				if componentHash, isSingleUse := singleUseComponents[refStr]; isSingleUse {
					// Не инлайним компоненты, используемые внутри других схем
					if isNested {
						return nil
					}
					// Находим компонент по хешу
					for _, ct := range componentTypes {
						section, ok := components[ct].(map[string]interface{})
						if !ok {
							continue
						}
						for name, component := range section {
							if r.hashComponent(component) == componentHash {
								// Заменяем $ref на полное содержимое
								componentCopy := r.deepCopy(component)
								if componentCopy != nil {
									for k, v := range componentCopy.(map[string]interface{}) {
										n[k] = v
									}
									delete(n, "$ref")
									// Удаляем компонент из components
									delete(section, name)
								}
								return nil
							}
						}
					}
				}
			}
		}
		// Рекурсивно обрабатываем все поля
		for _, v := range n {
			if err := r.replaceRefsWithInline(ctx, v, singleUseComponents, components, isNested); err != nil {
				return err
			}
		}
	case []interface{}:
		for _, item := range n {
			if err := r.replaceRefsWithInline(ctx, item, singleUseComponents, components, skipNested); err != nil {
				return err
			}
		}
	}
	return nil
}

// liftComponentRefs "поднимает" $ref в components: заменяет ссылки на реальное содержимое
func (r *ReferenceResolver) liftComponentRefs(ctx context.Context, data map[string]interface{}, config domain.Config) error {
	components, ok := data["components"].(map[string]interface{})
	if !ok {
		return nil
	}

	// Функция для проверки циклических ссылок
	var checkCycle func(currentType string, section map[string]interface{}, startName string, visited map[string]bool) bool
	checkCycle = func(currentType string, section map[string]interface{}, startName string, visited map[string]bool) bool {
		key := currentType + "/" + startName
		if visited[key] {
			return true // Цикл обнаружен
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

	// Проходим по всем типам компонентов
	for _, ct := range componentTypes {
		section, ok := components[ct].(map[string]interface{})
		if !ok {
			continue
		}

		// Собираем список компонентов для "поднятия" (чтобы не изменять map во время итерации)
		toLift := make(map[string]string) // name -> refName
		
		for name, component := range section {
			if componentMap, ok := component.(map[string]interface{}); ok {
				// Проверяем, является ли компонент только $ref
				if refVal, hasRef := componentMap["$ref"]; hasRef {
					if len(componentMap) == 1 {
						if refStr, ok := refVal.(string); ok {
							if strings.HasPrefix(refStr, "#/components/"+ct+"/") {
								// Внутренняя ссылка на компонент того же типа
								refName := strings.TrimPrefix(refStr, "#/components/"+ct+"/")
								
								// Проверка на самоссылку
								if refName == name {
									continue
								}

								// Проверка на циклическую ссылку
								visited := make(map[string]bool)
								if checkCycle(ct, section, refName, visited) {
									// Цикл обнаружен, не поднимаем
									continue
								}

								// Проверяем, что целевой компонент существует
								if targetComponent, exists := section[refName]; exists {
									// Проверяем, что целевой компонент не является только $ref (чтобы избежать цепочки)
									if targetMap, ok := targetComponent.(map[string]interface{}); ok {
										if _, hasTargetRef := targetMap["$ref"]; hasTargetRef {
											if len(targetMap) == 1 {
												// Целевой компонент тоже только $ref - не поднимаем, чтобы избежать цепочек
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

		// Выполняем "поднятие"
		for name, refName := range toLift {
			if targetComponent, exists := section[refName]; exists {
				// Проверяем, не является ли это самоссылкой
				if name == refName {
					// Это самоссылка, не поднимаем
					continue
				}
				// "Поднимаем" ссылку: заменяем на содержимое
				targetCopy := r.deepCopy(targetComponent)
				section[name] = targetCopy
			}
		}
	}

	return nil
}


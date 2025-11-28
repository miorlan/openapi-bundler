package resolver

import (
	"context"
	"encoding/json"
	"strings"
)

func (r *ReferenceResolver) replaceInlineSchemasWithRefs(ctx context.Context, data map[string]interface{}) error {
	paths, ok := data["paths"].(map[string]interface{})
	if !ok {
		return nil
	}

	components, ok := data["components"].(map[string]interface{})
	if !ok {
		return nil
	}

	schemas, ok := components["schemas"].(map[string]interface{})
	if !ok {
		return nil
	}

	hashToName := make(map[string]string)
	schemaByHash := make(map[string]map[string]interface{})
	for name, schema := range schemas {
		if schemaMap, ok := schema.(map[string]interface{}); ok {
			normalizedSchema := r.normalizeSchemaForComparisonWithSchemas(schemaMap, schemas)
			hash := r.hashComponent(normalizedSchema)
			if hash != "" {
				hashToName[hash] = name
				schemaByHash[hash] = normalizedSchema
			}
		}
	}

	for _, pathValue := range paths {
		if pathMap, ok := pathValue.(map[string]interface{}); ok {
			for _, methodValue := range pathMap {
				if methodMap, ok := methodValue.(map[string]interface{}); ok {
					if err := r.processOperationSchemas(ctx, methodMap, hashToName, schemaByHash, schemas); err != nil {
						return err
					}
				}
			}
		}
	}

	return nil
}

func (r *ReferenceResolver) processOperationSchemas(ctx context.Context, op map[string]interface{}, hashToName map[string]string, schemaByHash map[string]map[string]interface{}, schemas map[string]interface{}) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	if requestBody, ok := op["requestBody"].(map[string]interface{}); ok {
		if content, ok := requestBody["content"].(map[string]interface{}); ok {
			for _, mediaTypeValue := range content {
				if mediaType, ok := mediaTypeValue.(map[string]interface{}); ok {
					if schema, ok := mediaType["schema"].(map[string]interface{}); ok {
						if _, hasRef := schema["$ref"]; !hasRef {
							normalizedSchema := r.normalizeSchemaForComparisonWithSchemas(schema, schemas)
							hash := r.hashComponent(normalizedSchema)
							if name, exists := hashToName[hash]; exists {
								mediaType["schema"] = map[string]interface{}{
									"$ref": "#/components/schemas/" + name,
								}
							} else {
								for existingHash, existingName := range hashToName {
									existingNormalized := schemaByHash[existingHash]
									normalizedA := r.normalizeComponent(normalizedSchema)
									normalizedB := r.normalizeComponent(existingNormalized)
									dataA, errA := json.Marshal(normalizedA)
									dataB, errB := json.Marshal(normalizedB)
									if errA == nil && errB == nil {
										var unmarshaledA, unmarshaledB interface{}
										if err := json.Unmarshal(dataA, &unmarshaledA); err == nil {
											if err := json.Unmarshal(dataB, &unmarshaledB); err == nil {
												dataA, _ = json.Marshal(unmarshaledA)
												dataB, _ = json.Marshal(unmarshaledB)
												if string(dataA) == string(dataB) {
													mediaType["schema"] = map[string]interface{}{
														"$ref": "#/components/schemas/" + existingName,
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
			}
		}
	}

	if responses, ok := op["responses"].(map[string]interface{}); ok {
		for _, responseValue := range responses {
			if response, ok := responseValue.(map[string]interface{}); ok {
				if content, ok := response["content"].(map[string]interface{}); ok {
					for _, mediaTypeValue := range content {
						if mediaType, ok := mediaTypeValue.(map[string]interface{}); ok {
							if schema, ok := mediaType["schema"].(map[string]interface{}); ok {
								if _, hasRef := schema["$ref"]; !hasRef {
									normalizedSchema := r.normalizeSchemaForComparisonWithSchemas(schema, schemas)
									hash := r.hashComponent(normalizedSchema)
									if name, exists := hashToName[hash]; exists {
										mediaType["schema"] = map[string]interface{}{
											"$ref": "#/components/schemas/" + name,
										}
									} else {
										for existingHash, existingName := range hashToName {
											existingNormalized := schemaByHash[existingHash]
											normalizedA := r.normalizeComponent(normalizedSchema)
											normalizedB := r.normalizeComponent(existingNormalized)
											dataA, errA := json.Marshal(normalizedA)
											dataB, errB := json.Marshal(normalizedB)
											if errA == nil && errB == nil {
												var unmarshaledA, unmarshaledB interface{}
												if err := json.Unmarshal(dataA, &unmarshaledA); err == nil {
													if err := json.Unmarshal(dataB, &unmarshaledB); err == nil {
														dataA, _ = json.Marshal(unmarshaledA)
														dataB, _ = json.Marshal(unmarshaledB)
														if string(dataA) == string(dataB) {
															mediaType["schema"] = map[string]interface{}{
																"$ref": "#/components/schemas/" + existingName,
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
					}
				}
			}
		}
	}

	if parameters, ok := op["parameters"].([]interface{}); ok {
		for _, paramValue := range parameters {
			if param, ok := paramValue.(map[string]interface{}); ok {
				if schema, ok := param["schema"].(map[string]interface{}); ok {
					if _, hasRef := schema["$ref"]; !hasRef {
						normalizedSchema := r.normalizeSchemaForComparisonWithSchemas(schema, schemas)
						hash := r.hashComponent(normalizedSchema)
						if name, exists := hashToName[hash]; exists {
							param["schema"] = map[string]interface{}{
								"$ref": "#/components/schemas/" + name,
							}
						} else {
							for existingHash, existingName := range hashToName {
								existingNormalized := schemaByHash[existingHash]
								normalizedA := r.normalizeComponent(normalizedSchema)
								normalizedB := r.normalizeComponent(existingNormalized)
								dataA, errA := json.Marshal(normalizedA)
								dataB, errB := json.Marshal(normalizedB)
								if errA == nil && errB == nil {
									var unmarshaledA, unmarshaledB interface{}
									if err := json.Unmarshal(dataA, &unmarshaledA); err == nil {
										if err := json.Unmarshal(dataB, &unmarshaledB); err == nil {
											dataA, _ = json.Marshal(unmarshaledA)
											dataB, _ = json.Marshal(unmarshaledB)
											if string(dataA) == string(dataB) {
												param["schema"] = map[string]interface{}{
													"$ref": "#/components/schemas/" + existingName,
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
		}
	}

	return nil
}

func (r *ReferenceResolver) normalizeSchemaForComparison(schema map[string]interface{}) map[string]interface{} {
	return r.normalizeSchemaForComparisonWithSchemas(schema, nil)
}

func (r *ReferenceResolver) normalizeSchemaForComparisonWithSchemas(schema map[string]interface{}, availableSchemas map[string]interface{}) map[string]interface{} {
	return r.normalizeSchemaForComparisonWithSchemasRecursive(schema, availableSchemas, make(map[string]bool))
}

func (r *ReferenceResolver) normalizeSchemaForComparisonWithSchemasRecursive(schema map[string]interface{}, availableSchemas map[string]interface{}, visited map[string]bool) map[string]interface{} {
	if availableSchemas == nil {
		if components, ok := r.rootDoc["components"].(map[string]interface{}); ok {
			if schemasVal, ok := components["schemas"].(map[string]interface{}); ok {
				availableSchemas = schemasVal
			}
		}
	}
	
	normalized := make(map[string]interface{})
	for k, v := range schema {
		if k == "$ref" || k == "description" || k == "example" ||
			k == "title" || k == "deprecated" || k == "externalDocs" || k == "xml" ||
			k == "nullable" || k == "readOnly" || k == "writeOnly" {
			continue
		}
		if k == "properties" {
			if props, ok := v.(map[string]interface{}); ok {
				normalizedProps := make(map[string]interface{})
				for propName, propValue := range props {
					if propMap, ok := propValue.(map[string]interface{}); ok {
						if refVal, hasRef := propMap["$ref"]; hasRef {
							if refStr, ok := refVal.(string); ok {
								if strings.HasPrefix(refStr, "#/components/schemas/") {
									schemaName := strings.TrimPrefix(refStr, "#/components/schemas/")
									if availableSchemas != nil && !visited[schemaName] {
										if targetSchema, exists := availableSchemas[schemaName]; exists {
											if targetMap, ok := targetSchema.(map[string]interface{}); ok {
												visited[schemaName] = true
												resolved := r.normalizeSchemaForComparisonWithSchemasRecursive(targetMap, availableSchemas, visited)
												delete(visited, schemaName)
												if len(resolved) > 0 {
													normalizedProps[propName] = resolved
												}
											}
										} else {
											normalizedProps[propName] = map[string]interface{}{
												"$ref": refStr,
											}
										}
									} else {
										normalizedProps[propName] = map[string]interface{}{
											"$ref": refStr,
										}
									}
								} else {
									normalizedProps[propName] = r.normalizeSchemaForComparisonWithSchemasRecursive(propMap, availableSchemas, visited)
								}
							} else {
								normalizedProps[propName] = r.normalizeSchemaForComparisonWithSchemasRecursive(propMap, availableSchemas, visited)
							}
						} else {
							normalizedProp := r.normalizeSchemaForComparisonWithSchemasRecursive(propMap, availableSchemas, visited)
							if len(normalizedProp) > 0 {
								normalizedProps[propName] = normalizedProp
							}
						}
					} else {
						normalizedProps[propName] = propValue
					}
				}
				if len(normalizedProps) > 0 {
					normalized[k] = normalizedProps
				}
			} else {
				normalized[k] = v
			}
		} else if k == "items" || k == "additionalProperties" {
			if itemMap, ok := v.(map[string]interface{}); ok {
				if refVal, hasRef := itemMap["$ref"]; hasRef {
					if refStr, ok := refVal.(string); ok {
						if strings.HasPrefix(refStr, "#/components/schemas/") {
							schemaName := strings.TrimPrefix(refStr, "#/components/schemas/")
							if availableSchemas != nil && !visited[schemaName] {
								if targetSchema, exists := availableSchemas[schemaName]; exists {
									if targetMap, ok := targetSchema.(map[string]interface{}); ok {
										visited[schemaName] = true
										resolved := r.normalizeSchemaForComparisonWithSchemasRecursive(targetMap, availableSchemas, visited)
										delete(visited, schemaName)
										if len(resolved) > 0 {
											normalized[k] = resolved
										}
									}
								} else {
									normalized[k] = map[string]interface{}{
										"$ref": refStr,
									}
								}
							} else {
								normalized[k] = map[string]interface{}{
									"$ref": refStr,
								}
							}
						} else {
							normalized[k] = r.normalizeSchemaForComparisonWithSchemasRecursive(itemMap, availableSchemas, visited)
						}
					} else {
						normalized[k] = r.normalizeSchemaForComparisonWithSchemasRecursive(itemMap, availableSchemas, visited)
					}
				} else {
					normalized[k] = r.normalizeSchemaForComparisonWithSchemasRecursive(itemMap, availableSchemas, visited)
				}
			} else {
				normalized[k] = v
			}
		} else {
			normalized[k] = v
		}
	}
	return normalized
}

func (r *ReferenceResolver) resolveInternalRefsInPaths(ctx context.Context, data map[string]interface{}) error {
	paths, ok := data["paths"].(map[string]interface{})
	if !ok {
		return nil
	}

	components, ok := data["components"].(map[string]interface{})
	if !ok {
		return nil
	}

	schemas, ok := components["schemas"].(map[string]interface{})
	if !ok {
		return nil
	}

	for _, pathValue := range paths {
		if pathMap, ok := pathValue.(map[string]interface{}); ok {
			for _, methodValue := range pathMap {
				if methodMap, ok := methodValue.(map[string]interface{}); ok {
					if err := r.resolveInternalRefsInOperation(ctx, methodMap, schemas); err != nil {
						return err
					}
				}
			}
		}
	}

	return nil
}

func (r *ReferenceResolver) resolveInternalRefsInOperation(ctx context.Context, op map[string]interface{}, schemas map[string]interface{}) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	if requestBody, ok := op["requestBody"].(map[string]interface{}); ok {
		if content, ok := requestBody["content"].(map[string]interface{}); ok {
			for _, mediaTypeValue := range content {
				if mediaType, ok := mediaTypeValue.(map[string]interface{}); ok {
					if schema, ok := mediaType["schema"].(map[string]interface{}); ok {
						if err := r.resolveInternalRefsInSchema(ctx, schema, schemas); err != nil {
							return err
						}
					}
				}
			}
		}
	}

	if responses, ok := op["responses"].(map[string]interface{}); ok {
		for _, responseValue := range responses {
			if response, ok := responseValue.(map[string]interface{}); ok {
				if content, ok := response["content"].(map[string]interface{}); ok {
					for _, mediaTypeValue := range content {
						if mediaType, ok := mediaTypeValue.(map[string]interface{}); ok {
							if schema, ok := mediaType["schema"].(map[string]interface{}); ok {
								if err := r.resolveInternalRefsInSchema(ctx, schema, schemas); err != nil {
									return err
								}
							}
						}
					}
				}
			}
		}
	}

	return nil
}

func (r *ReferenceResolver) resolveInternalRefsInSchema(ctx context.Context, schema map[string]interface{}, schemas map[string]interface{}) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	if _, hasRef := schema["$ref"]; hasRef {
		return nil
	}

	if properties, ok := schema["properties"].(map[string]interface{}); ok {
		for _, propValue := range properties {
			if propMap, ok := propValue.(map[string]interface{}); ok {
				if _, hasRef := propMap["$ref"]; hasRef {
					continue
				}
				if err := r.resolveInternalRefsInSchema(ctx, propMap, schemas); err != nil {
					return err
				}
			}
		}
	}

	if items, ok := schema["items"].(map[string]interface{}); ok {
		if err := r.resolveInternalRefsInSchema(ctx, items, schemas); err != nil {
			return err
		}
	}

	if additionalProperties, ok := schema["additionalProperties"].(map[string]interface{}); ok {
		if err := r.resolveInternalRefsInSchema(ctx, additionalProperties, schemas); err != nil {
			return err
		}
	}

	return nil
}

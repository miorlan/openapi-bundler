package resolver

import (
	"context"
	"fmt"
	"strings"

	"github.com/miorlan/openapi-bundler/internal/domain"
)

var componentTypes = []string{"schemas", "responses", "parameters", "examples", "requestBodies", "headers", "securitySchemes", "links", "callbacks"}

type ReferenceResolver struct {
	fileLoader domain.FileLoader
	parser     domain.Parser
	visited    map[string]bool
	rootDoc    map[string]interface{}
	components map[string]map[string]interface{}
	componentRefs map[string]string
	componentCounter map[string]int
	pathsBaseDir string
	componentsBaseDir map[string]string
	fileCache map[string]interface{}
	componentHashes map[string]string
	rootBasePath string
	refToComponentName map[string]string
	componentUsageCount map[string]int
	externalRefUsageCount map[string]int
	schemaFileToName map[string]string
	processingComponentsIndex bool
	currentComponentName string
	// schemaIndex stores the mapping from schema name to its file path (from _index.yaml)
	schemaIndex map[string]string
	// schemaIndexBaseDir is the base directory for schema index files
	schemaIndexBaseDir string
	// usedSchemas tracks which schemas are actually used
	usedSchemas map[string]bool
}

func NewReferenceResolver(fileLoader domain.FileLoader, parser domain.Parser) domain.ReferenceResolver {
	components := make(map[string]map[string]interface{}, len(componentTypes))
	componentCounter := make(map[string]int, len(componentTypes))
	for _, ct := range componentTypes {
		components[ct] = make(map[string]interface{})
		componentCounter[ct] = 0
	}
	return &ReferenceResolver{
		fileLoader: fileLoader,
		parser:     parser,
		visited:    make(map[string]bool),
		components: components,
		componentRefs: make(map[string]string),
		componentCounter: componentCounter,
		componentsBaseDir: make(map[string]string),
		fileCache: make(map[string]interface{}),
		componentHashes: make(map[string]string),
		refToComponentName: make(map[string]string),
		componentUsageCount: make(map[string]int),
		externalRefUsageCount: make(map[string]int),
		schemaFileToName: make(map[string]string),
		schemaIndex: make(map[string]string),
		usedSchemas: make(map[string]bool),
	}
}

func (r *ReferenceResolver) ResolveAll(ctx context.Context, data map[string]interface{}, basePath string, config domain.Config) error {
	r.visited = make(map[string]bool)
	r.rootDoc = data
	r.rootBasePath = basePath
	r.pathsBaseDir = ""
	r.componentsBaseDir = make(map[string]string)
	r.fileCache = make(map[string]interface{})
	r.componentHashes = make(map[string]string)
	r.refToComponentName = make(map[string]string)
	r.componentUsageCount = make(map[string]int)
	r.externalRefUsageCount = make(map[string]int)
	r.schemaFileToName = make(map[string]string)
	r.schemaIndex = make(map[string]string)
	r.schemaIndexBaseDir = ""
	r.usedSchemas = make(map[string]bool)
	for _, ct := range componentTypes {
		r.components[ct] = make(map[string]interface{})
		r.componentCounter[ct] = 0
	}
	r.componentRefs = make(map[string]string)

	var components map[string]interface{}
	if c, ok := data["components"].(map[string]interface{}); ok {
		// Check if components itself is a $ref
		if refVal, hasRef := c["$ref"]; hasRef {
			if refStr, ok := refVal.(string); ok {
				// Resolve the components reference
				refPath := r.getRefPath(refStr, basePath)
				if refPath == "" {
					return fmt.Errorf("invalid components reference: %s", refStr)
				}
				
				content, err := r.loadAndParseFile(ctx, refStr, basePath, config)
				if err != nil {
					return fmt.Errorf("failed to load components file: %w", err)
				}
				
				// Extract components from the referenced file
				componentsMap := r.extractSection(content, "components")
				if componentsMap == nil {
					// If no components section found, try to use the whole content
					if m, ok := content.(map[string]interface{}); ok {
						componentsMap = m
					} else {
						return fmt.Errorf("failed to extract components from %s", refStr)
					}
				}
				
				// Process references within the loaded components
				sectionBaseDir := r.getSectionBaseDir(refPath)
				if err := r.replaceExternalRefs(ctx, componentsMap, sectionBaseDir, config, 0); err != nil {
					return fmt.Errorf("failed to process references in components: %w", err)
				}
				
				components = componentsMap
				data["components"] = components
			} else {
				components = c
			}
		} else {
			components = c
		}
	} else {
		components = make(map[string]interface{})
		data["components"] = components
	}

	for _, ct := range componentTypes {
		if _, ok := components[ct]; !ok {
			components[ct] = make(map[string]interface{})
		}
	}

	if err := r.expandSectionRefs(ctx, data, basePath, config); err != nil {
		return err
	}

	if config.MaxDepth == 0 {
		if err := r.preloadExternalFiles(ctx, data, basePath, config); err != nil {
			return fmt.Errorf("failed to preload external files: %w", err)
		}
	}

	if err := r.countExternalRefUsage(ctx, data, basePath, config, 0); err != nil {
		return err
	}

	if err := r.replaceExternalRefs(ctx, data, basePath, config, 0); err != nil {
		return err
	}

	for _, ct := range componentTypes {
		if section, ok := components[ct].(map[string]interface{}); ok {
			for name, component := range section {
				if componentMap, ok := component.(map[string]interface{}); ok {
					if refVal, hasRef := componentMap["$ref"]; hasRef {
						if len(componentMap) == 1 {
							if refStr, ok := refVal.(string); ok {
								expectedRef := "#/components/" + ct + "/" + name
								if refStr == expectedRef {
									delete(section, name)
								}
							}
						}
					}
				}
			}
		}
	}

	for _, ct := range componentTypes {
		if section, ok := components[ct].(map[string]interface{}); ok {
			for name, component := range r.components[ct] {
				if component == nil {
					continue
				}
				
				normalizedName := r.normalizeComponentName(name)
				
				componentHash := r.hashComponent(component)
				
				for existingName, existingComponent := range section {
					if existingMap, ok := existingComponent.(map[string]interface{}); ok {
						if refVal, hasRef := existingMap["$ref"]; hasRef {
							if len(existingMap) == 1 {
								if refStr, ok := refVal.(string); ok {
									expectedRef := "#/components/" + ct + "/" + existingName
									if refStr == expectedRef {
										if r.componentsEqual(component, existingComponent) || r.hashComponent(component) == r.hashComponent(existingComponent) {
											section[existingName] = component
											r.componentHashes[componentHash] = existingName
											if name != existingName {
												delete(r.components[ct], name)
											}
											goto nextComponent
										}
									}
								}
							}
						}
					}
				}
				
				if existingName, exists := r.componentHashes[componentHash]; exists {
					if existingName != normalizedName {
						continue
					}
				}
				
				if existing, exists := section[normalizedName]; !exists {
					section[normalizedName] = component
					r.componentHashes[componentHash] = normalizedName
				} else {
					if existingMap, ok := existing.(map[string]interface{}); ok {
						if refVal, hasRef := existingMap["$ref"]; hasRef {
							if len(existingMap) == 1 {
								if refStr, ok := refVal.(string); ok {
									expectedRef := "#/components/" + ct + "/" + normalizedName
									if refStr == expectedRef {
										section[normalizedName] = component
										r.componentHashes[componentHash] = normalizedName
										continue
									}
								}
								section[normalizedName] = component
								r.componentHashes[componentHash] = normalizedName
								continue
							}
						}
					}
					if r.componentsEqual(existing, component) {
						continue
					}
					uniqueName := r.ensureUniqueComponentName(normalizedName, section, ct)
					section[uniqueName] = component
					r.componentHashes[componentHash] = uniqueName
				}
			nextComponent:
			}
		}
	}

	if err := r.liftComponentRefs(ctx, data, config); err != nil {
		return err
	}

	r.cleanNilValues(data)

	// Note: We don't remove unused schemas to match swagger-cli behavior
	// swagger-cli keeps all schemas from _index.yaml even if not directly referenced

	for _, ct := range componentTypes {
		if section, ok := components[ct].(map[string]interface{}); ok {
			if len(section) == 0 {
				delete(components, ct)
			}
		}
	}

	if componentsMap, ok := data["components"].(map[string]interface{}); ok {
		if len(componentsMap) == 0 {
			delete(data, "components")
		}
	}

	return nil
}

// removeUnusedSchemas removes schemas that are not referenced anywhere in the document
func (r *ReferenceResolver) removeUnusedSchemas(data map[string]interface{}) {
	components, ok := data["components"].(map[string]interface{})
	if !ok {
		return
	}
	
	schemas, ok := components["schemas"].(map[string]interface{})
	if !ok {
		return
	}
	
	// Collect all schema references used in paths, responses, parameters, etc. (excluding schemas section)
	usedRefs := make(map[string]bool)
	r.collectSchemaRefsExcludingSchemas(data, usedRefs)
	
	// Now recursively add schemas that are referenced by other used schemas
	changed := true
	for changed {
		changed = false
		for schemaName := range usedRefs {
			if schema, exists := schemas[schemaName]; exists {
				newRefs := make(map[string]bool)
				r.collectSchemaRefsInNode(schema, newRefs)
				for ref := range newRefs {
					if !usedRefs[ref] {
						usedRefs[ref] = true
						changed = true
					}
				}
			}
		}
	}
	
	// Remove schemas that are not referenced
	for schemaName := range schemas {
		if !usedRefs[schemaName] {
			delete(schemas, schemaName)
			delete(r.components["schemas"], schemaName)
		}
	}
}

// collectSchemaRefsExcludingSchemas collects schema refs from everywhere except components.schemas
func (r *ReferenceResolver) collectSchemaRefsExcludingSchemas(data map[string]interface{}, usedRefs map[string]bool) {
	for key, value := range data {
		if key == "components" {
			if comps, ok := value.(map[string]interface{}); ok {
				for compKey, compValue := range comps {
					if compKey != "schemas" {
						r.collectSchemaRefsInNode(compValue, usedRefs)
					}
				}
			}
		} else {
			r.collectSchemaRefsInNode(value, usedRefs)
		}
	}
}

// collectSchemaRefsInNode recursively collects all schema references in a node
func (r *ReferenceResolver) collectSchemaRefsInNode(node interface{}, usedRefs map[string]bool) {
	switch n := node.(type) {
	case map[string]interface{}:
		if refVal, ok := n["$ref"]; ok {
			if refStr, ok := refVal.(string); ok {
				if strings.HasPrefix(refStr, "#/components/schemas/") {
					schemaName := strings.TrimPrefix(refStr, "#/components/schemas/")
					usedRefs[schemaName] = true
				}
			}
		}
		for _, value := range n {
			r.collectSchemaRefsInNode(value, usedRefs)
		}
	case []interface{}:
		for _, item := range n {
			r.collectSchemaRefsInNode(item, usedRefs)
		}
	}
}

func (r *ReferenceResolver) Resolve(ctx context.Context, ref string, basePath string, config domain.Config) (interface{}, error) {
	refPath := r.getRefPath(ref, basePath)
	if refPath == "" {
		return nil, &domain.ErrInvalidReference{Ref: ref}
	}
	return r.loadAndParseFile(ctx, ref, basePath, config)
}

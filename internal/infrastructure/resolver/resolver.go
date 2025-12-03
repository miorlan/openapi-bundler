package resolver

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/miorlan/openapi-bundler/internal/domain"
	"github.com/miorlan/openapi-bundler/internal/infrastructure/errors"
	"gopkg.in/yaml.v3"
)

// Resolver resolves OpenAPI references while preserving key order
type Resolver struct {
	fileLoader  domain.FileLoader
	fileCache   map[string]*yaml.Node
	visited     map[string]bool
	helper      *NodeHelper
	rootNode    *yaml.Node
	rootBaseDir string

	// Path tracking for JSON pointers
	currentPath []string

	// Base directories for different sections
	pathsBaseDir      string
	componentsBaseDir map[string]string

	// Mappings for reference resolution
	globalSchemaNames  map[string]bool
	schemaFileToName   map[string]string
	componentFileToRef map[string]string

	// Deduplication
	schemaHashToPath map[string]string

	// Collected schemas from external components.json files (ordered)
	collectedSchemas      map[string]*yaml.Node
	collectedSchemasOrder []string
}

// NewResolver creates a new Resolver
func NewResolver(fileLoader domain.FileLoader) *Resolver {
	return &Resolver{
		fileLoader: fileLoader,
		helper:     &NodeHelper{},
	}
}

// ResolveNode resolves all references in a yaml.Node
func (r *Resolver) ResolveNode(ctx context.Context, node *yaml.Node, basePath string, config domain.Config) error {
	r.reset(basePath)

	// Handle document node
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		node = node.Content[0]
	}

	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("root node must be a mapping")
	}

	r.rootNode = node
	return r.expandAndResolve(ctx, node, basePath, config)
}

// reset initializes all maps for a new resolution
func (r *Resolver) reset(basePath string) {
	r.rootBaseDir = basePath
	r.fileCache = make(map[string]*yaml.Node)
	r.visited = make(map[string]bool)
	r.currentPath = nil
	r.pathsBaseDir = ""
	r.componentsBaseDir = make(map[string]string)
	r.globalSchemaNames = make(map[string]bool)
	r.schemaFileToName = make(map[string]string)
	r.componentFileToRef = make(map[string]string)
	r.schemaHashToPath = make(map[string]string)
	r.collectedSchemas = make(map[string]*yaml.Node)
	r.collectedSchemasOrder = nil
}

// expandAndResolve expands sections and resolves references in the correct order
func (r *Resolver) expandAndResolve(ctx context.Context, node *yaml.Node, basePath string, config domain.Config) error {
	componentsNode := r.helper.GetMapValue(node, "components")
	pathsNode := r.helper.GetMapValue(node, "paths")

	// Phase 1: Expand all sections (load external files)
	if componentsNode != nil {
		if err := r.expandComponents(ctx, componentsNode, basePath, config); err != nil {
			return err
		}
	}

	if pathsNode != nil {
		if err := r.expandPaths(ctx, pathsNode, basePath, config); err != nil {
			return err
		}
	}

	// Phase 2: Resolve refs in components FIRST to get full schema content
	if componentsNode != nil {
		if err := r.resolveComponentRefs(ctx, componentsNode, config); err != nil {
			return err
		}
		// Register hashes for sub-elements (items, etc.) for deduplication in paths
		r.registerSchemaSubElements(componentsNode)
	}

	// Phase 3: Resolve refs in paths (schemas already resolved, deduplication will work)
	if pathsNode != nil {
		baseDir := r.pathsBaseDir
		if baseDir == "" {
			baseDir = basePath
		}
		r.pushPath("paths")
		if err := r.resolveRefs(ctx, pathsNode, baseDir, config, 0); err != nil {
			r.popPath()
			return err
		}
		r.popPath()
	}

	// Phase 4: Add collected schemas to components/schemas
	if len(r.collectedSchemas) > 0 {
		r.addCollectedSchemasToComponents(node)
	}

	return nil
}

// addCollectedSchemasToComponents adds collected schemas to the root components/schemas
func (r *Resolver) addCollectedSchemasToComponents(rootNode *yaml.Node) {
	// Get or create components node
	componentsNode := r.helper.GetMapValue(rootNode, "components")
	if componentsNode == nil {
		componentsNode = &yaml.Node{Kind: yaml.MappingNode}
		rootNode.Content = append(rootNode.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "components"},
			componentsNode,
		)
	}

	// Get or create schemas node
	schemasNode := r.helper.GetMapValue(componentsNode, "schemas")
	if schemasNode == nil {
		schemasNode = &yaml.Node{Kind: yaml.MappingNode}
		componentsNode.Content = append(componentsNode.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "schemas"},
			schemasNode,
		)
	}

	// Add collected schemas in order they were discovered
	for _, name := range r.collectedSchemasOrder {
		schema := r.collectedSchemas[name]
		// Check if schema already exists
		if r.helper.GetMapValue(schemasNode, name) != nil {
			continue
		}
		schemasNode.Content = append(schemasNode.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: name},
			schema,
		)
		r.globalSchemaNames[name] = true
	}
}

// expandComponents expands the components section
func (r *Resolver) expandComponents(ctx context.Context, node *yaml.Node, basePath string, config domain.Config) error {
	ref := r.helper.GetRef(node)
	if ref != "" {
		return r.expandComponentsRef(ctx, node, ref, basePath, config)
	}

	if node.Kind == yaml.MappingNode {
		if err := r.expandComponentSections(ctx, node, basePath, config); err != nil {
			return err
		}
		r.registerGlobalSchemas(node)
	}
	return nil
}

// expandComponentsRef expands a $ref pointing to components
func (r *Resolver) expandComponentsRef(ctx context.Context, node *yaml.Node, ref string, basePath string, config domain.Config) error {
	content, refPath, err := r.loadRefContent(ctx, ref, basePath, config)
	if err != nil {
		return fmt.Errorf("failed to load components: %w", err)
	}

	r.replaceNode(node, content)
	r.registerGlobalSchemas(node)

	baseDir := filepath.Dir(refPath)
	r.componentsBaseDir["schemas"] = baseDir

	if schemasNode := r.helper.GetMapValue(node, "schemas"); schemasNode != nil {
		r.buildSchemaMapping(schemasNode, baseDir)
	}

	return nil
}

// expandComponentSections expands individual component sections (schemas, parameters, etc.)
func (r *Resolver) expandComponentSections(ctx context.Context, node *yaml.Node, basePath string, config domain.Config) error {
	componentTypes := []string{"schemas", "responses", "parameters", "examples", "requestBodies", "headers", "securitySchemes", "links", "callbacks"}

	for _, ct := range componentTypes {
		sectionNode := r.helper.GetMapValue(node, ct)
		if sectionNode == nil {
			continue
		}

		ref := r.helper.GetRef(sectionNode)
		if ref == "" {
			continue
		}

		content, refPath, err := r.loadRefContent(ctx, ref, basePath, config)
		if err != nil {
			return fmt.Errorf("failed to expand components.%s: %w", ct, err)
		}

		baseDir := filepath.Dir(refPath)
		r.componentsBaseDir[ct] = baseDir
		r.buildComponentMapping(content, baseDir, ct)

		if ct == "schemas" {
			r.buildSchemaMapping(content, baseDir)
		}

		r.replaceNode(sectionNode, content)
	}

	return nil
}

// expandPaths expands the paths section
func (r *Resolver) expandPaths(ctx context.Context, node *yaml.Node, basePath string, config domain.Config) error {
	ref := r.helper.GetRef(node)
	if ref == "" {
		r.pathsBaseDir = basePath
		return nil
	}

	content, refPath, err := r.loadRefContent(ctx, ref, basePath, config)
	if err != nil {
		return fmt.Errorf("failed to expand paths: %w", err)
	}

	r.pathsBaseDir = filepath.Dir(refPath)
	r.replaceNode(node, content)
	return nil
}

// resolveComponentRefs resolves refs in all component sections
func (r *Resolver) resolveComponentRefs(ctx context.Context, node *yaml.Node, config domain.Config) error {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}

	r.pushPath("components")
	defer r.popPath()

	for i := 0; i < len(node.Content); i += 2 {
		if i+1 >= len(node.Content) {
			break
		}
		sectionName := node.Content[i].Value
		sectionNode := node.Content[i+1]

		baseDir := r.componentsBaseDir[sectionName]
		if baseDir == "" {
			baseDir = r.rootBaseDir
		}

		r.pushPath(sectionName)

		// First inline component definitions
		if err := r.inlineComponentDefinitions(ctx, sectionNode, baseDir, config); err != nil {
			r.popPath()
			return fmt.Errorf("failed to inline %s: %w", sectionName, err)
		}

		// Then resolve nested refs
		if err := r.resolveRefs(ctx, sectionNode, baseDir, config, 0); err != nil {
			r.popPath()
			return fmt.Errorf("failed to resolve refs in %s: %w", sectionName, err)
		}

		r.popPath()
	}

	return nil
}

// inlineComponentDefinitions inlines $refs at the component definition level
func (r *Resolver) inlineComponentDefinitions(ctx context.Context, node *yaml.Node, baseDir string, config domain.Config) error {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}

	for i := 0; i < len(node.Content); i += 2 {
		if i+1 >= len(node.Content) {
			break
		}
		componentValue := node.Content[i+1]

		ref := r.helper.GetRef(componentValue)
		if ref == "" || strings.HasPrefix(ref, "#") {
			continue
		}

		content, _, err := r.loadRefContent(ctx, ref, baseDir, config)
		if err != nil {
			return fmt.Errorf("failed to load component %s: %w", ref, err)
		}

		r.replaceNode(componentValue, content)
	}

	return nil
}

// resolveRefs recursively resolves all $ref in a node
func (r *Resolver) resolveRefs(ctx context.Context, node *yaml.Node, baseDir string, config domain.Config, depth int) error {
	return r.resolveRefsWithContext(ctx, node, baseDir, config, depth, nil)
}

// resolveRefsWithContext resolves refs with optional external file context
func (r *Resolver) resolveRefsWithContext(ctx context.Context, node *yaml.Node, baseDir string, config domain.Config, depth int, externalRoot *yaml.Node) error {
	if node == nil {
		return nil
	}

	if config.MaxDepth > 0 && depth > config.MaxDepth {
		return fmt.Errorf("maximum recursion depth exceeded")
	}

	switch node.Kind {
	case yaml.DocumentNode:
		for _, child := range node.Content {
			if err := r.resolveRefsWithContext(ctx, child, baseDir, config, depth, externalRoot); err != nil {
				return err
			}
		}

	case yaml.MappingNode:
		ref := r.helper.GetRef(node)
		if ref != "" {
			return r.resolveRef(ctx, node, ref, baseDir, config, depth, externalRoot)
		}

		// Process children with path tracking
		for i := 0; i < len(node.Content); i += 2 {
			if i+1 >= len(node.Content) {
				break
			}
			key := node.Content[i].Value
			r.pushPath(key)
			if err := r.resolveRefsWithContext(ctx, node.Content[i+1], baseDir, config, depth+1, externalRoot); err != nil {
				r.popPath()
				return err
			}
			r.popPath()
		}

	case yaml.SequenceNode:
		for _, child := range node.Content {
			if err := r.resolveRefsWithContext(ctx, child, baseDir, config, depth+1, externalRoot); err != nil {
				return err
			}
		}
	}

	return nil
}

// resolveRef resolves a single $ref
func (r *Resolver) resolveRef(ctx context.Context, node *yaml.Node, ref string, baseDir string, config domain.Config, depth int, externalRoot *yaml.Node) error {
	if strings.HasPrefix(ref, "#") {
		if externalRoot != nil {
			return r.resolveInternalRef(ctx, node, ref, baseDir, config, depth, externalRoot)
		}
		return nil // Skip internal refs to main document
	}

	return r.resolveExternalRef(ctx, node, ref, baseDir, config, depth)
}

// resolveInternalRef resolves an internal $ref within an external file
func (r *Resolver) resolveInternalRef(ctx context.Context, node *yaml.Node, ref string, baseDir string, config domain.Config, depth int, externalRoot *yaml.Node) error {
	fragment := strings.TrimPrefix(ref, "#")

	// If this is a schema reference, collect it and use global ref
	if strings.HasPrefix(fragment, "/components/schemas/") {
		schemaName := strings.TrimPrefix(fragment, "/components/schemas/")

		// If already global, just use internal ref
		if r.globalSchemaNames[schemaName] {
			r.helper.SetRef(node, "#/components/schemas/"+schemaName)
			return nil
		}

		// If already collected, use internal ref
		if _, exists := r.collectedSchemas[schemaName]; exists {
			r.helper.SetRef(node, "#/components/schemas/"+schemaName)
			return nil
		}

		// Collect schema from external file
		schemaContent := r.navigateToFragment(externalRoot, fragment)
		if schemaContent != nil {
			schemaContent = r.helper.CloneNode(schemaContent)
			// Resolve internal refs within the schema
			if err := r.resolveRefsWithContext(ctx, schemaContent, baseDir, config, depth+1, externalRoot); err != nil {
				return err
			}
			// Store for later addition to components/schemas
			r.collectedSchemas[schemaName] = schemaContent
			r.collectedSchemasOrder = append(r.collectedSchemasOrder, schemaName)
			// Convert to internal ref
			r.helper.SetRef(node, "#/components/schemas/"+schemaName)
			return nil
		}
	}

	content := r.navigateToFragment(externalRoot, fragment)
	if content == nil {
		return fmt.Errorf("internal reference %s not found", ref)
	}

	content = r.helper.CloneNode(content)

	if err := r.resolveRefsWithContext(ctx, content, baseDir, config, depth+1, externalRoot); err != nil {
		return err
	}

	if r.tryDeduplicateSchema(node, content) {
		return nil
	}

	r.replaceNode(node, content)
	return nil
}

// resolveExternalRef resolves an external $ref
func (r *Resolver) resolveExternalRef(ctx context.Context, node *yaml.Node, ref string, baseDir string, config domain.Config, depth int) error {
	refPath := r.getRefPath(ref, baseDir)
	if refPath == "" {
		return fmt.Errorf("invalid reference: %s", ref)
	}

	absPath, _ := filepath.Abs(refPath)

	// Check for circular references
	visitKey := fmt.Sprintf("%s:%p", absPath, node)
	if r.visited[visitKey] {
		return nil
	}
	r.visited[visitKey] = true
	defer delete(r.visited, visitKey)

	// Try to convert to internal ref
	if internalRef := r.tryConvertToInternalRef(absPath); internalRef != "" {
		r.helper.SetRef(node, internalRef)
		return nil
	}

	// Load content
	content, err := r.loadFile(ctx, refPath, config)
	if err != nil {
		return fmt.Errorf("failed to resolve %s: %w", ref, err)
	}

	if content.Kind == yaml.DocumentNode && len(content.Content) > 0 {
		content = content.Content[0]
	}

	// Handle fragment
	fragment := ""
	if idx := strings.Index(ref, "#"); idx >= 0 {
		fragment = ref[idx+1:]
	}

	return r.resolveRefWithFragment(ctx, node, content, fragment, refPath, config, depth)
}

// resolveRefWithFragment resolves a ref with an optional fragment
func (r *Resolver) resolveRefWithFragment(ctx context.Context, node *yaml.Node, content *yaml.Node, fragment string, refPath string, config domain.Config, depth int) error {
	newBaseDir := filepath.Dir(refPath)

	// Handle component schema references - collect and convert to internal ref
	if strings.HasPrefix(fragment, "/components/schemas/") {
		schemaName := strings.TrimPrefix(fragment, "/components/schemas/")

		// If already global, just use internal ref
		if r.globalSchemaNames[schemaName] {
			r.helper.SetRef(node, "#/components/schemas/"+schemaName)
			return nil
		}

		// Collect schema from external file
		schemaContent := r.navigateToFragment(content, fragment)
		if schemaContent != nil {
			schemaContent = r.helper.CloneNode(schemaContent)
			// Resolve internal refs within the schema
			if err := r.resolveRefsWithContext(ctx, schemaContent, newBaseDir, config, depth+1, content); err != nil {
				return err
			}
			// Store for later addition to components/schemas (preserve order)
			if _, exists := r.collectedSchemas[schemaName]; !exists {
				r.collectedSchemas[schemaName] = schemaContent
				r.collectedSchemasOrder = append(r.collectedSchemasOrder, schemaName)
			}
			// Convert to internal ref
			r.helper.SetRef(node, "#/components/schemas/"+schemaName)
			return nil
		}
	}

	// Navigate to fragment if present
	if fragment != "" && fragment != "/" {
		fragmentContent := r.navigateToFragment(content, fragment)
		if fragmentContent == nil {
			return fmt.Errorf("fragment %s not found", fragment)
		}

		// For component refs, use external file context
		if strings.HasPrefix(fragment, "/components/") {
			fragmentContent = r.helper.CloneNode(fragmentContent)
			if err := r.resolveRefsWithContext(ctx, fragmentContent, newBaseDir, config, depth+1, content); err != nil {
				return err
			}
			if r.tryDeduplicateSchema(node, fragmentContent) {
				return nil
			}
			r.replaceNode(node, fragmentContent)
			return nil
		}

		content = fragmentContent
	}

	content = r.helper.CloneNode(content)

	if err := r.resolveRefs(ctx, content, newBaseDir, config, depth+1); err != nil {
		return err
	}

	if r.tryDeduplicateSchema(node, content) {
		return nil
	}

	r.replaceNode(node, content)
	return nil
}

// tryConvertToInternalRef tries to convert an absolute path to an internal ref
func (r *Resolver) tryConvertToInternalRef(absPath string) string {
	// Check schema mapping
	if name, ok := r.schemaFileToName[absPath]; ok {
		return "#/components/schemas/" + name
	}
	if name, ok := r.schemaFileToName[strings.TrimSuffix(absPath, filepath.Ext(absPath))]; ok {
		return "#/components/schemas/" + name
	}

	// Check component mapping
	if ref, ok := r.componentFileToRef[absPath]; ok {
		return ref
	}
	if ref, ok := r.componentFileToRef[strings.TrimSuffix(absPath, filepath.Ext(absPath))]; ok {
		return ref
	}

	return ""
}

// tryDeduplicateSchema checks if content was already seen and uses a ref instead
// Only deduplicates to #/components/schemas/... refs (oapi-codegen compatible)
func (r *Resolver) tryDeduplicateSchema(node *yaml.Node, content *yaml.Node) bool {
	hash := r.hashNode(content)

	if existingPath, ok := r.schemaHashToPath[hash]; ok {
		// Only use refs that point to components/schemas (oapi-codegen compatible)
		if strings.HasPrefix(existingPath, "#/components/schemas/") {
			r.helper.SetRef(node, existingPath)
			return true
		}
	}

	// Only register schemas under #/components/schemas/ for deduplication
	if currentPath := r.getCurrentJSONPointer(); currentPath != "" {
		if strings.HasPrefix(currentPath, "#/components/schemas/") {
			r.schemaHashToPath[hash] = currentPath
		}
	}

	return false
}

// Helper methods

// loadRefContent loads content from a reference, handling fragments
func (r *Resolver) loadRefContent(ctx context.Context, ref string, baseDir string, config domain.Config) (*yaml.Node, string, error) {
	refPath := r.getRefPath(ref, baseDir)
	if refPath == "" {
		return nil, "", fmt.Errorf("invalid reference: %s", ref)
	}

	content, err := r.loadFile(ctx, refPath, config)
	if err != nil {
		return nil, "", err
	}

	if content.Kind == yaml.DocumentNode && len(content.Content) > 0 {
		content = content.Content[0]
	}

	// Handle fragment
	if idx := strings.Index(ref, "#"); idx >= 0 {
		fragment := ref[idx+1:]
		if fragment != "" && fragment != "/" {
			content = r.navigateToFragment(content, fragment)
			if content == nil {
				return nil, "", fmt.Errorf("fragment %s not found in %s", fragment, ref)
			}
		}
	}

	return r.helper.CloneNode(content), refPath, nil
}

// replaceNode replaces the content of dst with src
func (r *Resolver) replaceNode(dst, src *yaml.Node) {
	dst.Kind = src.Kind
	dst.Content = src.Content
	dst.Value = src.Value
	dst.Tag = src.Tag
	dst.Style = src.Style
}

// registerGlobalSchemas registers all schemas as global
func (r *Resolver) registerGlobalSchemas(componentsNode *yaml.Node) {
	if componentsNode == nil || componentsNode.Kind != yaml.MappingNode {
		return
	}

	schemasNode := r.helper.GetMapValue(componentsNode, "schemas")
	if schemasNode == nil || schemasNode.Kind != yaml.MappingNode {
		return
	}

	for i := 0; i < len(schemasNode.Content); i += 2 {
		if i+1 >= len(schemasNode.Content) {
			break
		}
		r.globalSchemaNames[schemasNode.Content[i].Value] = true
	}
}

// registerSchemaSubElements registers hashes for schema sub-elements
func (r *Resolver) registerSchemaSubElements(componentsNode *yaml.Node) {
	if componentsNode == nil || componentsNode.Kind != yaml.MappingNode {
		return
	}

	schemasNode := r.helper.GetMapValue(componentsNode, "schemas")
	if schemasNode == nil || schemasNode.Kind != yaml.MappingNode {
		return
	}

	for i := 0; i < len(schemasNode.Content); i += 2 {
		if i+1 >= len(schemasNode.Content) {
			break
		}
		schemaName := schemasNode.Content[i].Value
		schemaNode := schemasNode.Content[i+1]

		r.registerSubElement(schemaNode, "items", "#/components/schemas/"+schemaName+"/items")
		r.registerSubElement(schemaNode, "additionalProperties", "#/components/schemas/"+schemaName+"/additionalProperties")

		// Register property items
		if propsNode := r.helper.GetMapValue(schemaNode, "properties"); propsNode != nil && propsNode.Kind == yaml.MappingNode {
			for j := 0; j < len(propsNode.Content); j += 2 {
				if j+1 >= len(propsNode.Content) {
					break
				}
				propName := propsNode.Content[j].Value
				propNode := propsNode.Content[j+1]
				r.registerSubElement(propNode, "items", "#/components/schemas/"+schemaName+"/properties/"+propName+"/items")
			}
		}
	}
}

// registerSubElement registers a hash for a sub-element if it exists
func (r *Resolver) registerSubElement(parent *yaml.Node, key string, path string) {
	node := r.helper.GetMapValue(parent, key)
	if node != nil && node.Kind == yaml.MappingNode {
		hash := r.hashNode(node)
		if _, exists := r.schemaHashToPath[hash]; !exists {
			r.schemaHashToPath[hash] = path
		}
	}
}

// buildSchemaMapping builds mapping from file paths to schema names
func (r *Resolver) buildSchemaMapping(schemasNode *yaml.Node, baseDir string) {
	if schemasNode == nil || schemasNode.Kind != yaml.MappingNode {
		return
	}

	for i := 0; i < len(schemasNode.Content); i += 2 {
		if i+1 >= len(schemasNode.Content) {
			break
		}
		schemaName := schemasNode.Content[i].Value
		schemaValue := schemasNode.Content[i+1]

		r.mapRefToName(schemaValue, baseDir, schemaName, r.schemaFileToName)
		r.mapNameToFile(baseDir, schemaName, r.schemaFileToName)
	}
}

// buildComponentMapping builds mapping from file paths to internal refs
func (r *Resolver) buildComponentMapping(sectionNode *yaml.Node, baseDir string, componentType string) {
	if sectionNode == nil || sectionNode.Kind != yaml.MappingNode {
		return
	}

	for i := 0; i < len(sectionNode.Content); i += 2 {
		if i+1 >= len(sectionNode.Content) {
			break
		}
		componentName := sectionNode.Content[i].Value
		componentValue := sectionNode.Content[i+1]
		internalRef := fmt.Sprintf("#/components/%s/%s", componentType, componentName)

		r.mapRefToName(componentValue, baseDir, internalRef, r.componentFileToRef)
		r.mapNameToFileWithRef(baseDir, componentName, internalRef, r.componentFileToRef)
	}
}

// mapRefToName maps a $ref path to a name/ref
func (r *Resolver) mapRefToName(node *yaml.Node, baseDir string, name string, mapping map[string]string) {
	ref := r.helper.GetRef(node)
	if ref == "" {
		return
	}

	refPath := r.getRefPath(ref, baseDir)
	if refPath == "" {
		return
	}

	absPath, _ := filepath.Abs(refPath)
	mapping[absPath] = name
	mapping[strings.TrimSuffix(absPath, filepath.Ext(absPath))] = name
}

// mapNameToFile maps possible file paths to a name
func (r *Resolver) mapNameToFile(baseDir string, name string, mapping map[string]string) {
	r.mapNameToFileWithRef(baseDir, name, name, mapping)
}

// mapNameToFileWithRef maps possible file paths to a ref
func (r *Resolver) mapNameToFileWithRef(baseDir string, name string, ref string, mapping map[string]string) {
	for _, ext := range []string{".json", ".yaml", ".yml"} {
		p := filepath.Join(baseDir, name+ext)
		if absPath, err := filepath.Abs(p); err == nil {
			if _, err := os.Stat(p); err == nil {
				mapping[absPath] = ref
				mapping[strings.TrimSuffix(absPath, ext)] = ref
			}
		}
	}
}

// navigateToFragment navigates to a JSON pointer fragment
func (r *Resolver) navigateToFragment(node *yaml.Node, fragment string) *yaml.Node {
	if node == nil || fragment == "" {
		return node
	}

	fragment = strings.TrimPrefix(fragment, "/")
	if fragment == "" {
		return node
	}

	current := node
	for _, part := range strings.Split(fragment, "/") {
		if part == "" {
			continue
		}
		// Unescape JSON pointer
		part = strings.ReplaceAll(part, "~1", "/")
		part = strings.ReplaceAll(part, "~0", "~")

		if current.Kind != yaml.MappingNode {
			return nil
		}
		current = r.helper.GetMapValue(current, part)
		if current == nil {
			return nil
		}
	}

	return current
}

// loadFile loads and parses a file with caching
func (r *Resolver) loadFile(ctx context.Context, path string, config domain.Config) (*yaml.Node, error) {
	path = filepath.Clean(path)

	if cached, ok := r.fileCache[path]; ok {
		return cached, nil
	}

	data, err := r.fileLoader.Load(ctx, path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &errors.ErrFileNotFound{Path: path}
		}
		return nil, fmt.Errorf("failed to load file: %w", err)
	}

	var node yaml.Node
	if err := yaml.Unmarshal(data, &node); err != nil {
		return nil, fmt.Errorf("failed to parse file: %w", err)
	}

	r.fileCache[path] = &node
	return &node, nil
}

// getRefPath resolves a reference path relative to baseDir
func (r *Resolver) getRefPath(ref string, baseDir string) string {
	if strings.HasPrefix(ref, "#") {
		return ""
	}

	refPath := ref
	if idx := strings.Index(ref, "#"); idx >= 0 {
		refPath = ref[:idx]
	}

	if strings.HasPrefix(refPath, "http://") || strings.HasPrefix(refPath, "https://") {
		return refPath
	}

	if strings.HasPrefix(refPath, "./") || strings.HasPrefix(refPath, "../") || !strings.HasPrefix(refPath, "/") {
		return filepath.Join(baseDir, refPath)
	}

	return refPath
}

// hashNode computes a hash of a yaml.Node
func (r *Resolver) hashNode(node *yaml.Node) string {
	if node == nil {
		return ""
	}
	var buf strings.Builder
	r.writeNodeHash(&buf, node)
	hash := sha256.Sum256([]byte(buf.String()))
	return hex.EncodeToString(hash[:])
}

// writeNodeHash writes node content for hashing
func (r *Resolver) writeNodeHash(buf *strings.Builder, node *yaml.Node) {
	if node == nil {
		return
	}
	switch node.Kind {
	case yaml.DocumentNode:
		for _, child := range node.Content {
			r.writeNodeHash(buf, child)
		}
	case yaml.MappingNode:
		buf.WriteString("{")
		for i := 0; i < len(node.Content); i += 2 {
			if i+1 >= len(node.Content) {
				break
			}
			buf.WriteString(node.Content[i].Value)
			buf.WriteString(":")
			r.writeNodeHash(buf, node.Content[i+1])
			buf.WriteString(",")
		}
		buf.WriteString("}")
	case yaml.SequenceNode:
		buf.WriteString("[")
		for _, child := range node.Content {
			r.writeNodeHash(buf, child)
			buf.WriteString(",")
		}
		buf.WriteString("]")
	case yaml.ScalarNode:
		buf.WriteString(node.Value)
	}
}

// getCurrentJSONPointer returns the current JSON pointer path
func (r *Resolver) getCurrentJSONPointer() string {
	if len(r.currentPath) == 0 {
		return ""
	}
	var parts []string
	for _, p := range r.currentPath {
		escaped := strings.ReplaceAll(p, "~", "~0")
		escaped = strings.ReplaceAll(escaped, "/", "~1")
		if strings.HasPrefix(p, "/") {
			escaped = url.PathEscape(escaped)
		}
		parts = append(parts, escaped)
	}
	return "#/" + strings.Join(parts, "/")
}

// pushPath adds a path segment
func (r *Resolver) pushPath(segment string) {
	r.currentPath = append(r.currentPath, segment)
}

// popPath removes the last path segment
func (r *Resolver) popPath() {
	if len(r.currentPath) > 0 {
		r.currentPath = r.currentPath[:len(r.currentPath)-1]
	}
}

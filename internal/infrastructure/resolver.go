package infrastructure

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/miorlan/openapi-bundler/internal/domain"
)

type ReferenceResolver struct {
	fileLoader domain.FileLoader
	parser     domain.Parser
	visited    map[string]bool
	rootDoc    map[string]interface{}
	components map[string]map[string]interface{}
	componentRefs map[string]string
	componentCounter map[string]int
}

func NewReferenceResolver(fileLoader domain.FileLoader, parser domain.Parser) domain.ReferenceResolver {
	componentTypes := []string{"schemas", "responses", "parameters", "examples", "requestBodies", "headers", "securitySchemes", "links", "callbacks"}
	components := make(map[string]map[string]interface{})
	componentCounter := make(map[string]int)
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
	}
}

func (r *ReferenceResolver) ResolveAll(ctx context.Context, data map[string]interface{}, basePath string, config domain.Config) error {
	r.visited = make(map[string]bool)
	r.rootDoc = data
	componentTypes := []string{"schemas", "responses", "parameters", "examples", "requestBodies", "headers", "securitySchemes", "links", "callbacks"}
	for _, ct := range componentTypes {
		r.components[ct] = make(map[string]interface{})
		r.componentCounter[ct] = 0
	}
	r.componentRefs = make(map[string]string)

	var components map[string]interface{}
	if c, ok := data["components"].(map[string]interface{}); ok {
		components = c
	} else {
		components = make(map[string]interface{})
		data["components"] = components
	}

	for _, ct := range componentTypes {
		if _, ok := components[ct]; !ok {
			components[ct] = make(map[string]interface{})
		}
	}

	if err := r.replaceExternalRefs(ctx, data, basePath, config, 0); err != nil {
		return err
	}

	for _, ct := range componentTypes {
		if section, ok := components[ct].(map[string]interface{}); ok {
			for name, component := range r.components[ct] {
				if _, exists := section[name]; !exists {
					section[name] = component
				}
			}
		}
	}

	return nil
}

func (r *ReferenceResolver) Resolve(ctx context.Context, ref string, basePath string, config domain.Config) (interface{}, error) {
	return r.resolveRef(ctx, ref, basePath, config, 0)
}

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
			return nil
		}
		if _, hasOneOf := n["oneOf"]; hasOneOf {
			return nil
		}
		if _, hasAnyOf := n["anyOf"]; hasAnyOf {
			return nil
		}

		if refVal, ok := n["$ref"]; ok {
			refStr, ok := refVal.(string)
			if !ok {
				return &domain.ErrInvalidReference{Ref: fmt.Sprintf("%v", refVal)}
			}

			if strings.HasPrefix(refStr, "#") {
				return nil
			}

			internalRef, err := r.resolveAndReplaceExternalRef(ctx, refStr, baseDir, config, depth)
			if err != nil {
				return fmt.Errorf("failed to replace external ref %s: %w", refStr, err)
			}

			if internalRef != "" {
				n["$ref"] = internalRef
			}

			return nil
		}

		skipFields := map[string]bool{
			"properties":          true,
			"items":               true,
			"additionalProperties": true,
			"patternProperties":   true,
		}

		for k, v := range n {
			if skipFields[k] {
				continue
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
	var content interface{}
	if err := r.parser.Unmarshal(data, &content, format); err != nil {
		return "", fmt.Errorf("failed to parse file: %w", err)
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
				componentTypes := []string{"schemas", "responses", "parameters", "examples", "requestBodies", "headers", "securitySchemes", "links", "callbacks"}
				for _, ct := range componentTypes {
					if section, ok := comps[ct].(map[string]interface{}); ok && len(section) == 1 {
						for _, comp := range section {
							componentContent = comp
							componentType = ct
							break
						}
						break
					}
				}
				if componentContent == nil {
					componentContent = contentMap
				}
			} else {
				componentContent = contentMap
			}
		} else {
			componentContent = content
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

	if existingRef, ok := r.componentRefs[visitedKey]; ok {
		return existingRef, nil
	}

	componentCopy := r.deepCopy(componentContent)
	if err := r.replaceExternalRefs(ctx, componentCopy, nextBaseDir, config, depth+1); err != nil {
		return "", fmt.Errorf("failed to process component: %w", err)
	}

	if _, exists := section[componentName]; !exists {
		r.components[componentType][componentName] = componentCopy
	} else {
		if _, existsInCollected := r.components[componentType][componentName]; !existsInCollected {
			r.components[componentType][componentName] = componentCopy
		}
	}

	internalRef := "#/components/" + componentType + "/" + componentName
	r.componentRefs[visitedKey] = internalRef

	return internalRef, nil
}

func (r *ReferenceResolver) getPreferredComponentName(ref, fragment, componentType string) string {
	if fragment != "" {
		parts := strings.Split(strings.TrimPrefix(fragment, "#/"), "/")
		if len(parts) >= 3 && parts[0] == "components" && parts[1] == componentType {
			return parts[2]
		} else if len(parts) >= 1 {
			return parts[len(parts)-1]
		}
	}

	refPath := strings.Split(ref, "#")[0]
	if refPath != "" {
		baseName := filepath.Base(refPath)
		ext := filepath.Ext(baseName)
		if ext != "" {
			return strings.TrimSuffix(baseName, ext)
		}
		return baseName
	}

	r.componentCounter[componentType]++
	baseName := componentType[:len(componentType)-1]
	if len(baseName) > 0 {
		baseName = strings.ToUpper(baseName[:1]) + baseName[1:]
	}
	return fmt.Sprintf("%s%d", baseName, r.componentCounter[componentType])
}

func (r *ReferenceResolver) ensureUniqueComponentName(preferredName string, section map[string]interface{}, componentType string) string {
	name := preferredName
	counter := 0
	for {
		if _, exists := section[name]; !exists {
			if _, existsInCollected := r.components[componentType][name]; !existsInCollected {
				return name
			}
		}
		counter++
		name = fmt.Sprintf("%s%d", preferredName, counter)
	}
}

func (r *ReferenceResolver) resolveRef(ctx context.Context, ref string, baseDir string, config domain.Config, depth int) (interface{}, error) {
	return nil, nil
}

func (r *ReferenceResolver) resolveJSONPointer(doc interface{}, pointer string) (interface{}, error) {
	if !strings.HasPrefix(pointer, "#/") {
		return nil, fmt.Errorf("invalid JSON pointer format: %s", pointer)
	}

	path := pointer[2:]
	if path == "" {
		return doc, nil
	}

	parts := strings.Split(path, "/")
	current := doc

	for _, part := range parts {
		part = strings.ReplaceAll(part, "~1", "/")
		part = strings.ReplaceAll(part, "~0", "~")

		switch v := current.(type) {
		case map[string]interface{}:
			var ok bool
			current, ok = v[part]
			if !ok {
				return nil, fmt.Errorf("JSON pointer path not found: %s (missing key: %s)", pointer, part)
			}
		case []interface{}:
			idx := -1
			if _, err := fmt.Sscanf(part, "%d", &idx); err == nil && idx >= 0 && idx < len(v) {
				current = v[idx]
			} else {
				return nil, fmt.Errorf("JSON pointer path not found: %s (invalid array index: %s)", pointer, part)
			}
		default:
			return nil, fmt.Errorf("JSON pointer path not found: %s (cannot traverse %T)", pointer, current)
		}
	}

	return current, nil
}

func (r *ReferenceResolver) getRefPath(ref string, baseDir string) string {
	ref = strings.Split(ref, "#")[0]
	if ref == "" {
		return ""
	}

	if strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") {
		return ref
	}

	if strings.HasPrefix(baseDir, "http://") || strings.HasPrefix(baseDir, "https://") {
		if strings.HasPrefix(ref, "/") {
			idx := strings.Index(baseDir[8:], "/")
			if idx == -1 {
				return baseDir + ref
			}
			baseURL := baseDir[:idx+8]
			return baseURL + ref
		}
		return baseDir + ref
	}

	var result string
	if filepath.IsAbs(ref) {
		result = ref
	} else if strings.HasPrefix(ref, ".") {
		result = filepath.Join(baseDir, ref)
	} else {
		result = filepath.Join(baseDir, ref)
	}

	return filepath.Clean(result)
}

func (r *ReferenceResolver) deepCopy(src interface{}) interface{} {
	switch v := src.(type) {
	case map[string]interface{}:
		dst := make(map[string]interface{})
		for k, val := range v {
			dst[k] = r.deepCopy(val)
		}
		return dst
	case []interface{}:
		dst := make([]interface{}, len(v))
		for i, val := range v {
			dst[i] = r.deepCopy(val)
		}
		return dst
	default:
		return v
	}
}

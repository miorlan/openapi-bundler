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
	schemas    map[string]interface{}
	schemaRefs map[string]string
}

func NewReferenceResolver(fileLoader domain.FileLoader, parser domain.Parser) domain.ReferenceResolver {
	return &ReferenceResolver{
		fileLoader: fileLoader,
		parser:     parser,
		visited:    make(map[string]bool),
		schemas:    make(map[string]interface{}),
		schemaRefs: make(map[string]string),
	}
}

func (r *ReferenceResolver) ResolveAll(ctx context.Context, data map[string]interface{}, basePath string, config domain.Config) error {
	r.visited = make(map[string]bool)
	r.rootDoc = data
	r.schemas = make(map[string]interface{})
	r.schemaRefs = make(map[string]string)

	if components, ok := data["components"].(map[string]interface{}); ok {
		if _, ok := components["schemas"]; !ok {
			components["schemas"] = make(map[string]interface{})
		}
	} else {
		data["components"] = map[string]interface{}{
			"schemas": make(map[string]interface{}),
		}
	}

	if err := r.collectSchemasAndReplaceRefs(ctx, data, basePath, config, 0, false); err != nil {
		return err
	}

	components := data["components"].(map[string]interface{})
	schemas := components["schemas"].(map[string]interface{})
	for name, schema := range r.schemas {
		if _, exists := schemas[name]; !exists {
			schemas[name] = schema
		}
	}

	return nil
}

func (r *ReferenceResolver) Resolve(ctx context.Context, ref string, basePath string, config domain.Config) (interface{}, error) {
	return r.resolveRef(ctx, ref, basePath, config, 0)
}

func (r *ReferenceResolver) collectSchemasAndReplaceRefs(ctx context.Context, node interface{}, baseDir string, config domain.Config, depth int, inSchemas bool) error {
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

		if _, isSchemas := n["schemas"]; isSchemas {
			inSchemas = true
		}

		if refVal, ok := n["$ref"]; ok {
			refStr, ok := refVal.(string)
			if !ok {
				return &domain.ErrInvalidReference{Ref: fmt.Sprintf("%v", refVal)}
			}

			if strings.HasPrefix(refStr, "#") {
				return nil
			}

			internalRef, _, err := r.replaceExternalRef(ctx, refStr, baseDir, config, depth)
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

			childInSchemas := inSchemas
			if k == "schemas" {
				childInSchemas = true
			}
			if err := r.collectSchemasAndReplaceRefs(ctx, v, baseDir, config, depth, childInSchemas); err != nil {
				return fmt.Errorf("failed to process field %s: %w", k, err)
			}
		}

	case []interface{}:
		for i, item := range n {
			if err := r.collectSchemasAndReplaceRefs(ctx, item, baseDir, config, depth, inSchemas); err != nil {
				return fmt.Errorf("failed to process array item %d: %w", i, err)
			}
		}
	}

	return nil
}

func (r *ReferenceResolver) replaceExternalRef(ctx context.Context, ref string, baseDir string, config domain.Config, depth int) (string, interface{}, error) {
	refParts := strings.SplitN(ref, "#", 2)
	refPath := refParts[0]
	fragment := ""
	if len(refParts) > 1 {
		fragment = "#" + refParts[1]
	}

	if refPath == "" {
		return "", nil, nil
	}

	refPath = r.getRefPath(refPath, baseDir)
	if refPath == "" {
		return "", nil, &domain.ErrInvalidReference{Ref: ref}
	}

	if !strings.HasPrefix(refPath, "http://") && !strings.HasPrefix(refPath, "https://") {
		refPath = filepath.Clean(refPath)
	}

	visitedKey := refPath + fragment
	if r.visited[visitedKey] {
		if internalRef, ok := r.schemaRefs[visitedKey]; ok {
			schemaName := r.getSchemaName(ref, fragment)
			if schemaName != "" {
				if schema, exists := r.schemas[schemaName]; exists {
					return internalRef, schema, nil
				}
			}
			return internalRef, nil, nil
		}
		return "", nil, &domain.ErrCircularReference{Path: visitedKey}
	}
	r.visited[visitedKey] = true
	defer delete(r.visited, visitedKey)

	data, err := r.fileLoader.Load(ctx, refPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil, &ErrFileNotFound{Path: refPath}
		}
		return "", nil, fmt.Errorf("failed to load file: %w", err)
	}

	if config.MaxFileSize > 0 && int64(len(data)) > config.MaxFileSize {
		return "", nil, fmt.Errorf("file size %d exceeds maximum allowed size %d", len(data), config.MaxFileSize)
	}

	format := domain.DetectFormat(refPath)
	var content interface{}
	if err := r.parser.Unmarshal(data, &content, format); err != nil {
		return "", nil, fmt.Errorf("failed to parse file: %w", err)
	}

	var nextBaseDir string
	if strings.HasPrefix(refPath, "http://") || strings.HasPrefix(refPath, "https://") {
		nextBaseDir = refPath[:strings.LastIndex(refPath, "/")+1]
	} else {
		nextBaseDir = filepath.Dir(refPath)
	}

	if fragment != "" {
		extracted, err := r.resolveJSONPointer(content, fragment)
		if err != nil {
			return "", nil, fmt.Errorf("failed to resolve fragment %s: %w", fragment, err)
		}

		schemaName := r.getSchemaName(ref, fragment)
		if schemaName == "" {
			return "", nil, fmt.Errorf("cannot determine schema name for ref: %s", ref)
		}

		if existingRef, ok := r.schemaRefs[visitedKey]; ok {
			if schema, exists := r.schemas[schemaName]; exists {
				return existingRef, schema, nil
			}
			return existingRef, nil, nil
		}

		if err := r.collectSchemasAndReplaceRefs(ctx, extracted, nextBaseDir, config, depth+1, false); err != nil {
			return "", nil, fmt.Errorf("failed to process schema: %w", err)
		}

		r.schemas[schemaName] = extracted
		internalRef := "#/components/schemas/" + schemaName
		r.schemaRefs[visitedKey] = internalRef

		return internalRef, extracted, nil
	}

	if contentMap, ok := content.(map[string]interface{}); ok {
		if components, ok := contentMap["components"].(map[string]interface{}); ok {
			if schemas, ok := components["schemas"].(map[string]interface{}); ok {
				for schemaName, schema := range schemas {
					if err := r.collectSchemasAndReplaceRefs(ctx, schema, nextBaseDir, config, depth+1, false); err != nil {
						return "", nil, fmt.Errorf("failed to process schema %s: %w", schemaName, err)
					}

					if _, exists := r.schemas[schemaName]; !exists {
						r.schemas[schemaName] = schema
					}
				}
			}
		}

		if err := r.collectSchemasAndReplaceRefs(ctx, content, nextBaseDir, config, depth+1, false); err != nil {
			return "", nil, fmt.Errorf("failed to process external file: %w", err)
		}
	}

	return "", nil, nil
}

func (r *ReferenceResolver) getSchemaName(ref, fragment string) string {
	if fragment != "" {
		parts := strings.Split(strings.TrimPrefix(fragment, "#/"), "/")
		if len(parts) >= 3 && parts[0] == "components" && parts[1] == "schemas" {
			return parts[2]
		}
		if len(parts) >= 1 {
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

	return ""
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

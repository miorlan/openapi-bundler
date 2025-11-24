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
	rootDoc    interface{} // Корневой документ для разрешения внутренних ссылок
}

func NewReferenceResolver(fileLoader domain.FileLoader, parser domain.Parser) domain.ReferenceResolver {
	return &ReferenceResolver{
		fileLoader: fileLoader,
		parser:     parser,
		visited:    make(map[string]bool),
	}
}

func (r *ReferenceResolver) ResolveAll(ctx context.Context, data map[string]interface{}, basePath string, config domain.Config) error {
	r.visited = make(map[string]bool)
	r.rootDoc = data // Сохраняем корневой документ для разрешения внутренних ссылок
	return r.resolveRefs(ctx, data, basePath, config, 0)
}

func (r *ReferenceResolver) Resolve(ctx context.Context, ref string, basePath string, config domain.Config) (interface{}, error) {
	return r.resolveRef(ctx, ref, basePath, config, 0)
}

func (r *ReferenceResolver) resolveRefs(ctx context.Context, node interface{}, baseDir string, config domain.Config, depth int) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	if config.MaxDepth > 0 && depth >= config.MaxDepth {
		return fmt.Errorf("maximum recursion depth %d exceeded", config.MaxDepth)
	}

	switch n := node.(type) {
	case map[string]interface{}:
		if refVal, ok := n["$ref"]; ok {
			refStr, ok := refVal.(string)
			if !ok {
				return &domain.ErrInvalidReference{Ref: fmt.Sprintf("%v", refVal)}
			}

			if strings.HasPrefix(refStr, "#") {
				if r.rootDoc == nil {
					return fmt.Errorf("cannot resolve internal reference %s: root document not available", refStr)
				}
				resolved, err := r.resolveJSONPointer(r.rootDoc, refStr)
				if err != nil {
					return fmt.Errorf("failed to resolve internal reference %s: %w", refStr, err)
				}
				resolvedCopy := r.deepCopy(resolved)
				for k := range n {
					delete(n, k)
				}
				if resolvedMap, ok := resolvedCopy.(map[string]interface{}); ok {
					for k, v := range resolvedMap {
						n[k] = v
					}
					return r.resolveRefs(ctx, n, baseDir, config, depth+1)
				}
				return nil
			}

			resolved, err := r.resolveRef(ctx, refStr, baseDir, config, depth)
			if err != nil {
				return fmt.Errorf("failed to resolve $ref %s: %w", refStr, err)
			}

			if resolved == nil {
				return nil
			}

			for k := range n {
				delete(n, k)
			}
			if resolvedMap, ok := resolved.(map[string]interface{}); ok {
				for k, v := range resolvedMap {
					n[k] = v
				}
				refPath := r.getRefPath(refStr, baseDir)
				if refPath != "" {
					var nextBaseDir string
					if strings.HasPrefix(refPath, "http://") || strings.HasPrefix(refPath, "https://") {
						nextBaseDir = refPath[:strings.LastIndex(refPath, "/")+1]
					} else {
						refPath = filepath.Clean(refPath)
						nextBaseDir = filepath.Dir(refPath)
					}
					return r.resolveRefs(ctx, n, nextBaseDir, config, depth+1)
				}
				return r.resolveRefs(ctx, n, baseDir, config, depth+1)
			}
			return nil
		}

		for k, v := range n {
			if err := r.resolveRefs(ctx, v, baseDir, config, depth); err != nil {
				return fmt.Errorf("failed to resolve refs in %s: %w", k, err)
			}
		}

	case []interface{}:
		for i, item := range n {
			if err := r.resolveRefs(ctx, item, baseDir, config, depth); err != nil {
				return fmt.Errorf("failed to resolve refs in array item %d: %w", i, err)
			}
		}
	}

	return nil
}

func (r *ReferenceResolver) resolveRef(ctx context.Context, ref string, baseDir string, config domain.Config, depth int) (interface{}, error) {
	refParts := strings.SplitN(ref, "#", 2)
	refPath := refParts[0]
	fragment := ""
	if len(refParts) > 1 {
		fragment = "#" + refParts[1]
	}

	if refPath == "" && fragment != "" {
		return nil, nil
	}

	if strings.HasPrefix(ref, "#") {
		return nil, nil
	}

	refPath = r.getRefPath(refPath, baseDir)
	if refPath == "" {
		return nil, &domain.ErrInvalidReference{Ref: ref}
	}

	if !strings.HasPrefix(refPath, "http://") && !strings.HasPrefix(refPath, "https://") {
		refPath = filepath.Clean(refPath)
	}

	visitedKey := refPath + fragment
	if r.visited[visitedKey] {
		return nil, &domain.ErrCircularReference{Path: visitedKey}
	}
	r.visited[visitedKey] = true
	defer delete(r.visited, visitedKey)

	data, err := r.fileLoader.Load(ctx, refPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &ErrFileNotFound{Path: refPath}
		}
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	if config.MaxFileSize > 0 && int64(len(data)) > config.MaxFileSize {
		return nil, fmt.Errorf("file size %d exceeds maximum allowed size %d", len(data), config.MaxFileSize)
	}

	format := domain.DetectFormat(refPath)

	var content interface{}
	if err := r.parser.Unmarshal(data, &content, format); err != nil {
		return nil, fmt.Errorf("failed to parse file: %w", err)
	}

	if fragment != "" {
		extracted, err := r.resolveJSONPointer(content, fragment)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve fragment %s: %w", fragment, err)
		}
		content = extracted
	}

	var nextBaseDir string
	if strings.HasPrefix(refPath, "http://") || strings.HasPrefix(refPath, "https://") {
		nextBaseDir = refPath[:strings.LastIndex(refPath, "/")+1]
	} else {
		nextBaseDir = filepath.Dir(refPath)
	}

	originalRootDoc := r.rootDoc
	if contentMap, ok := content.(map[string]interface{}); ok {
		r.rootDoc = contentMap
	}

	if err := r.resolveRefs(ctx, content, nextBaseDir, config, depth+1); err != nil {
		r.rootDoc = originalRootDoc
		return nil, fmt.Errorf("failed to resolve refs in loaded file: %w", err)
	}

	r.rootDoc = originalRootDoc

	return content, nil
}

// resolveJSONPointer извлекает значение по JSON Pointer (RFC 6901)
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

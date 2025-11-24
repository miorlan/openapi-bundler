package infrastructure

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/miorlan/openapi-bundler/internal/domain"
)

// ReferenceResolver реализует разрешение $ref ссылок
type ReferenceResolver struct {
	fileLoader domain.FileLoader
	parser     domain.Parser
	visited    map[string]bool
}

// NewReferenceResolver создает новый resolver
func NewReferenceResolver(fileLoader domain.FileLoader, parser domain.Parser) domain.ReferenceResolver {
	return &ReferenceResolver{
		fileLoader: fileLoader,
		parser:     parser,
		visited:    make(map[string]bool),
	}
}

// ResolveAll разрешает все $ref ссылки в данных
func (r *ReferenceResolver) ResolveAll(ctx context.Context, data map[string]interface{}, basePath string, config domain.Config) error {
	r.visited = make(map[string]bool)
	return r.resolveRefs(ctx, data, basePath, config, 0)
}

// Resolve разрешает одну ссылку
func (r *ReferenceResolver) Resolve(ctx context.Context, ref string, basePath string, config domain.Config) (interface{}, error) {
	return r.resolveRef(ctx, ref, basePath, config, 0)
}

// resolveRefs рекурсивно разрешает все $ref ссылки
func (r *ReferenceResolver) resolveRefs(ctx context.Context, node interface{}, baseDir string, config domain.Config, depth int) error {
	// Проверяем контекст
	if ctx.Err() != nil {
		return ctx.Err()
	}

	// Проверяем глубину рекурсии
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

// resolveRef разрешает одну $ref ссылку
func (r *ReferenceResolver) resolveRef(ctx context.Context, ref string, baseDir string, config domain.Config, depth int) (interface{}, error) {
	if strings.HasPrefix(ref, "#") {
		return nil, nil
	}

	refPath := r.getRefPath(ref, baseDir)
	if refPath == "" {
		return nil, &domain.ErrInvalidReference{Ref: ref}
	}

	if !strings.HasPrefix(refPath, "http://") && !strings.HasPrefix(refPath, "https://") {
		refPath = filepath.Clean(refPath)
	}

	if r.visited[refPath] {
		return nil, &domain.ErrCircularReference{Path: refPath}
	}
	r.visited[refPath] = true
	defer delete(r.visited, refPath)

	data, err := r.fileLoader.Load(ctx, refPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &ErrFileNotFound{Path: refPath}
		}
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Проверяем размер файла
	if config.MaxFileSize > 0 && int64(len(data)) > config.MaxFileSize {
		return nil, fmt.Errorf("file size %d exceeds maximum allowed size %d", len(data), config.MaxFileSize)
	}

	format := domain.DetectFormat(refPath)

	var content interface{}
	if err := r.parser.Unmarshal(data, &content, format); err != nil {
		return nil, fmt.Errorf("failed to parse file: %w", err)
	}

	var nextBaseDir string
	if strings.HasPrefix(refPath, "http://") || strings.HasPrefix(refPath, "https://") {
		nextBaseDir = refPath[:strings.LastIndex(refPath, "/")+1]
	} else {
		nextBaseDir = filepath.Dir(refPath)
	}

	if err := r.resolveRefs(ctx, content, nextBaseDir, config, depth+1); err != nil {
		return nil, fmt.Errorf("failed to resolve refs in loaded file: %w", err)
	}

	return content, nil
}

// getRefPath получает абсолютный путь к файлу по $ref ссылке
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

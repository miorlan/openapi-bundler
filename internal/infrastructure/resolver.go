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
	schemas    map[string]interface{} // Собранные схемы из внешних файлов
	schemaRefs map[string]string      // Маппинг внешних ссылок на внутренние
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

	// Инициализируем components/schemas если его нет
	if components, ok := data["components"].(map[string]interface{}); ok {
		if _, ok := components["schemas"]; !ok {
			components["schemas"] = make(map[string]interface{})
		}
	} else {
		data["components"] = map[string]interface{}{
			"schemas": make(map[string]interface{}),
		}
	}

	// Собираем все внешние схемы и заменяем ссылки
	if err := r.collectSchemasAndReplaceRefs(ctx, data, basePath, config, 0, false); err != nil {
		return err
	}

	// Добавляем собранные схемы в components/schemas
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

// collectSchemasAndReplaceRefs собирает схемы из внешних файлов и заменяет внешние $ref на внутренние
// inSchemas указывает, находимся ли мы внутри components/schemas
func (r *ReferenceResolver) collectSchemasAndReplaceRefs(ctx context.Context, node interface{}, baseDir string, config domain.Config, depth int, inSchemas bool) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	if config.MaxDepth > 0 && depth >= config.MaxDepth {
		return fmt.Errorf("maximum recursion depth %d exceeded", config.MaxDepth)
	}

	switch n := node.(type) {
	case map[string]interface{}:
		// Пропускаем allOf/oneOf/anyOf - не разворачиваем их
		if _, hasAllOf := n["allOf"]; hasAllOf {
			// Обрабатываем только ссылки внутри allOf, но не разворачиваем сам allOf
			if allOf, ok := n["allOf"].([]interface{}); ok {
				for _, item := range allOf {
					if err := r.collectSchemasAndReplaceRefs(ctx, item, baseDir, config, depth+1, inSchemas); err != nil {
						return err
					}
				}
			}
			return nil
		}
		if _, hasOneOf := n["oneOf"]; hasOneOf {
			if oneOf, ok := n["oneOf"].([]interface{}); ok {
				for _, item := range oneOf {
					if err := r.collectSchemasAndReplaceRefs(ctx, item, baseDir, config, depth+1, inSchemas); err != nil {
						return err
					}
				}
			}
			return nil
		}
		if _, hasAnyOf := n["anyOf"]; hasAnyOf {
			if anyOf, ok := n["anyOf"].([]interface{}); ok {
				for _, item := range anyOf {
					if err := r.collectSchemasAndReplaceRefs(ctx, item, baseDir, config, depth+1, inSchemas); err != nil {
						return err
					}
				}
			}
			return nil
		}

		// Проверяем, находимся ли мы в components/schemas
		if _, isSchemas := n["schemas"]; isSchemas {
			inSchemas = true
		}

		// Обрабатываем $ref
		if refVal, ok := n["$ref"]; ok {
			refStr, ok := refVal.(string)
			if !ok {
				return &domain.ErrInvalidReference{Ref: fmt.Sprintf("%v", refVal)}
			}

			// Внутренние ссылки оставляем как есть
			if strings.HasPrefix(refStr, "#") {
				// Рекурсивно обрабатываем содержимое по ссылке, но не инлайним
				return nil
			}

		// Внешние ссылки заменяем на внутренние
		internalRef, schemaContent, err := r.replaceExternalRef(ctx, refStr, baseDir, config, depth)
		if err != nil {
			return fmt.Errorf("failed to replace external ref %s: %w", refStr, err)
		}

		if internalRef != "" {
			// Если текущий узел содержит только $ref (и больше ничего),
			// и мы находимся в components/schemas, заменяем содержимое вместо создания ссылки
			if len(n) == 1 && schemaContent != nil && inSchemas {
				// Заменяем содержимое
				for k := range n {
					delete(n, k)
				}
				if schemaMap, ok := schemaContent.(map[string]interface{}); ok {
					for k, v := range schemaMap {
						n[k] = v
					}
				}
				return nil
			}
			// Создаем ссылку
			n["$ref"] = internalRef
		}

		return nil
		}

		// Рекурсивно обрабатываем все остальные поля
		for k, v := range n {
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

// replaceExternalRef заменяет внешнюю ссылку на внутреннюю и собирает схемы
// Возвращает внутреннюю ссылку и содержимое схемы
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
		// Если уже обработали, возвращаем сохраненную внутреннюю ссылку
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

	// Загружаем внешний файл
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

	// Если есть фрагмент, извлекаем только нужную схему
	if fragment != "" {
		extracted, err := r.resolveJSONPointer(content, fragment)
		if err != nil {
			return "", nil, fmt.Errorf("failed to resolve fragment %s: %w", fragment, err)
		}

		// Определяем имя схемы из фрагмента
		schemaName := r.getSchemaName(ref, fragment)
		if schemaName == "" {
			return "", nil, fmt.Errorf("cannot determine schema name for ref: %s", ref)
		}

		// Если схема уже собрана, возвращаем ссылку на неё
		if existingRef, ok := r.schemaRefs[visitedKey]; ok {
			if schema, exists := r.schemas[schemaName]; exists {
				return existingRef, schema, nil
			}
			return existingRef, nil, nil
		}

		// Обрабатываем ссылки внутри извлеченной схемы
		if err := r.collectSchemasAndReplaceRefs(ctx, extracted, nextBaseDir, config, depth+1, false); err != nil {
			return "", nil, fmt.Errorf("failed to process schema: %w", err)
		}

		// Сохраняем схему
		r.schemas[schemaName] = extracted
		internalRef := "#/components/schemas/" + schemaName
		r.schemaRefs[visitedKey] = internalRef

		return internalRef, extracted, nil
	}

	// Если нет фрагмента, загружаем весь файл и извлекаем все схемы из components/schemas
	if contentMap, ok := content.(map[string]interface{}); ok {
		if components, ok := contentMap["components"].(map[string]interface{}); ok {
			if schemas, ok := components["schemas"].(map[string]interface{}); ok {
				// Обрабатываем все схемы из внешнего файла
				for schemaName, schema := range schemas {
					// Обрабатываем ссылки внутри схемы
					if err := r.collectSchemasAndReplaceRefs(ctx, schema, nextBaseDir, config, depth+1, false); err != nil {
						return "", nil, fmt.Errorf("failed to process schema %s: %w", schemaName, err)
					}

					// Сохраняем схему, если её еще нет
					if _, exists := r.schemas[schemaName]; !exists {
						r.schemas[schemaName] = schema
					}
				}
			}
		}

		// Обрабатываем остальные ссылки в файле (например, в paths)
		if err := r.collectSchemasAndReplaceRefs(ctx, content, nextBaseDir, config, depth+1, false); err != nil {
			return "", nil, fmt.Errorf("failed to process external file: %w", err)
		}
	}

	// Если ссылка указывает на весь файл без фрагмента, возвращаем пустую строку
	// (схемы уже добавлены в r.schemas)
	return "", nil, nil
}

// getSchemaName извлекает имя схемы из ссылки
func (r *ReferenceResolver) getSchemaName(ref, fragment string) string {
	if fragment != "" {
		// Извлекаем имя из фрагмента, например: #/components/schemas/User -> User
		parts := strings.Split(strings.TrimPrefix(fragment, "#/"), "/")
		if len(parts) >= 3 && parts[0] == "components" && parts[1] == "schemas" {
			return parts[2]
		}
		// Если фрагмент указывает на схему напрямую
		if len(parts) >= 1 {
			return parts[len(parts)-1]
		}
	}

	// Если нет фрагмента, пытаемся извлечь имя из пути файла
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
	// Этот метод используется только для Resolve, но теперь мы не инлайним ссылки
	// Возвращаем nil, так как основная логика в collectSchemasAndReplaceRefs
	return nil, nil
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

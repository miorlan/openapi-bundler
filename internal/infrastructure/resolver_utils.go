package infrastructure

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

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

// normalizeComponentName нормализует имя компонента: убирает специальные символы, приводит к валидному формату
func (r *ReferenceResolver) normalizeComponentName(name string) string {
	if name == "" {
		return name
	}
	
	// Убираем префиксы типа ".._.._schemas_" или "schemas_" из начала имени
	// Это остатки от старой логики построения имён из путей
	// Убираем все повторяющиеся префиксы ".._" или "../"
	for strings.HasPrefix(name, ".._") {
		name = strings.TrimPrefix(name, ".._")
	}
	for strings.HasPrefix(name, "../") {
		name = strings.TrimPrefix(name, "../")
	}
	
	// Убираем префиксы типа "schemas_", "responses_" и т.д.
	for _, ct := range componentTypes {
		prefix := ct + "_"
		for strings.HasPrefix(name, prefix) {
			name = strings.TrimPrefix(name, prefix)
		}
	}
	
	// Убираем специальные символы, оставляем только буквы, цифры и подчёркивания
	var result strings.Builder
	for _, char := range name {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '_' {
			result.WriteRune(char)
		} else {
			// Заменяем все остальные символы на подчёркивание, но не добавляем подряд
			if result.Len() > 0 {
				lastChar := result.String()[result.Len()-1]
				if lastChar != '_' {
					result.WriteRune('_')
				}
			}
		}
	}
	
	normalized := result.String()
	
	// Убираем множественные подчёркивания
	for strings.Contains(normalized, "__") {
		normalized = strings.ReplaceAll(normalized, "__", "_")
	}
	
	// Убираем подчёркивания в начале и конце
	normalized = strings.Trim(normalized, "_")
	
	// Если имя пустое после нормализации, генерируем имя
	if normalized == "" {
		r.componentCounter["schemas"]++
		return fmt.Sprintf("Component%d", r.componentCounter["schemas"])
	}
	
	// Первый символ должен быть буквой (не цифрой и не подчёркиванием)
	if len(normalized) > 0 {
		first := normalized[0]
		if (first >= '0' && first <= '9') || first == '_' {
			normalized = "C" + strings.TrimPrefix(normalized, "_")
		}
	}
	
	return normalized
}

// getPreferredComponentName определяет имя компонента по стратегии:
// 1. Если $ref указывает на файл (./schemas/EmployeeFullInfo.yaml) → использовать имя файла
// 2. Если $ref указывает на #/components/schemas/FooBar → использовать FooBar
// 3. Если создаётся из inline-схемы с title → использовать title
// 4. Иначе → Inline_<path>_...
func (r *ReferenceResolver) getPreferredComponentName(ref, fragment, componentType string, componentContent interface{}) string {
	var name string
	
	// 1. Если есть фрагмент с компонентом (#/components/schemas/FooBar) → используем имя из фрагмента
	if fragment != "" {
		parts := strings.Split(strings.TrimPrefix(fragment, "#/"), "/")
		if len(parts) >= 3 && parts[0] == "components" && parts[1] == componentType {
			// Используем оригинальное имя схемы из фрагмента
			name = parts[2]
			return r.normalizeComponentName(name)
		} else if len(parts) >= 1 {
			name = parts[len(parts)-1]
			return r.normalizeComponentName(name)
		}
	}
	
	// 2. Если $ref указывает на файл → используем имя файла
	refPath := strings.Split(ref, "#")[0]
	if refPath != "" {
		// Извлекаем имя файла без расширения
		baseName := filepath.Base(refPath)
		ext := filepath.Ext(baseName)
		if ext != "" {
			baseName = strings.TrimSuffix(baseName, ext)
		}
		if baseName != "" {
			return r.normalizeComponentName(baseName)
		}
	}
	
	// 3. Если это inline-схема с title → используем title
	if componentContent != nil {
		if schemaMap, ok := componentContent.(map[string]interface{}); ok {
			if title, hasTitle := schemaMap["title"]; hasTitle {
				if titleStr, ok := title.(string); ok && titleStr != "" {
					return r.normalizeComponentName(titleStr)
				}
			}
		}
	}
	
	// 4. Генерируем имя на основе пути (Inline_<path>_...)
	if refPath != "" {
		// Создаём имя на основе пути к файлу
		pathParts := strings.Split(strings.Trim(refPath, "./"), "/")
		var pathName strings.Builder
		pathName.WriteString("Inline")
		for _, part := range pathParts {
			if part != "" {
				pathName.WriteString("_")
				pathName.WriteString(strings.TrimSuffix(part, filepath.Ext(part)))
			}
		}
		if pathName.Len() > len("Inline") {
			return r.normalizeComponentName(pathName.String())
		}
	}
	
	// Последний резерв: генерируем имя
	r.componentCounter[componentType]++
	return fmt.Sprintf("Inline_%s%d", componentType[:len(componentType)-1], r.componentCounter[componentType])
}

func (r *ReferenceResolver) ensureUniqueComponentName(preferredName string, section map[string]interface{}, componentType string) string {
	name := preferredName
	counter := 0
	for {
		// Проверяем уникальность в финальной секции и в собранных компонентах
		if _, exists := section[name]; !exists {
			if _, existsInCollected := r.components[componentType][name]; !existsInCollected {
				return name
			}
		}
		counter++
		name = fmt.Sprintf("%s%d", preferredName, counter)
	}
}

func (r *ReferenceResolver) hashComponent(component interface{}) string {
	normalized := r.normalizeComponent(component)
	data, err := json.Marshal(normalized)
	if err != nil {
		return ""
	}
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash)
}

func (r *ReferenceResolver) normalizeComponent(component interface{}) interface{} {
	switch v := component.(type) {
	case map[string]interface{}:
		normalized := make(map[string]interface{})
		for k, val := range v {
			if k == "$ref" {
				continue
			}
			normalized[k] = r.normalizeComponent(val)
		}
		return normalized
	case []interface{}:
		normalized := make([]interface{}, len(v))
		for i, val := range v {
			normalized[i] = r.normalizeComponent(val)
		}
		return normalized
	default:
		return v
	}
}

func (r *ReferenceResolver) componentsEqual(a, b interface{}) bool {
	normalizedA := r.normalizeComponent(a)
	normalizedB := r.normalizeComponent(b)
	dataA, errA := json.Marshal(normalizedA)
	dataB, errB := json.Marshal(normalizedB)
	if errA != nil || errB != nil {
		return false
		}
	return string(dataA) == string(dataB)
}

func (r *ReferenceResolver) cleanNilValues(node interface{}) interface{} {
	switch n := node.(type) {
	case map[string]interface{}:
		for k, v := range n {
			if v == nil {
				delete(n, k)
				continue
			}
			cleaned := r.cleanNilValues(v)
			if cleaned == nil {
				delete(n, k)
				continue
			}
			n[k] = cleaned
		}
		return n
	case []interface{}:
		result := make([]interface{}, 0, len(n))
		for _, item := range n {
			if item == nil {
				continue
			}
			cleaned := r.cleanNilValues(item)
			if cleaned != nil {
				result = append(result, cleaned)
			}
		}
		return result
	default:
		return n
	}
}

// resolveInternalRef разрешает внутреннюю ссылку и возвращает содержимое компонента
func (r *ReferenceResolver) resolveInternalRef(ref string) (interface{}, error) {
	if !strings.HasPrefix(ref, "#/components/") {
		return nil, fmt.Errorf("not an internal component reference: %s", ref)
	}
	
	path := strings.TrimPrefix(ref, "#/")
	return r.resolveJSONPointer(r.rootDoc, "#/"+path)
}



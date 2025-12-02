package resolver

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
)

var (
	builderPool = sync.Pool{
		New: func() interface{} {
			return &strings.Builder{}
		},
	}
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
		dst := make(map[string]interface{}, len(v))
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

func (r *ReferenceResolver) normalizeComponentName(name string) string {
	if name == "" {
		return name
	}

	for strings.HasPrefix(name, ".._") {
		name = strings.TrimPrefix(name, ".._")
	}
	for strings.HasPrefix(name, "../") {
		name = strings.TrimPrefix(name, "../")
	}

	hasInlinePrefix := strings.HasPrefix(name, "Inline_")
	if !hasInlinePrefix {
		for _, ct := range componentTypes {
			prefix := ct + "_"
			for strings.HasPrefix(name, prefix) {
				name = strings.TrimPrefix(name, prefix)
			}
		}
	}

	result := builderPool.Get().(*strings.Builder)
	defer func() {
		result.Reset()
		builderPool.Put(result)
	}()
	for _, char := range name {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '_' || char == '-' {
			result.WriteRune(char)
		} else {
			if result.Len() > 0 {
				lastChar := result.String()[result.Len()-1]
				if lastChar != '_' && lastChar != '-' {
					result.WriteRune('_')
				}
			}
		}
	}

	normalized := result.String()

	for strings.Contains(normalized, "__") {
		normalized = strings.ReplaceAll(normalized, "__", "_")
	}

	normalized = strings.Trim(normalized, "_")

	if normalized == "" {
		return "Component"
	}

	if len(normalized) > 0 {
		first := normalized[0]
		if (first >= '0' && first <= '9') || first == '_' {
			normalized = "C" + strings.TrimPrefix(normalized, "_")
		}
	}

	return normalized
}

func (r *ReferenceResolver) getPreferredComponentName(ref, fragment, componentType string, componentContent interface{}) string {
	if fragment != "" {
		parts := strings.Split(strings.TrimPrefix(fragment, "#/"), "/")
		if len(parts) >= 3 && parts[0] == "components" && parts[1] == componentType {
			return r.normalizeComponentName(parts[2])
		}
		if len(parts) >= 1 {
			return r.normalizeComponentName(parts[len(parts)-1])
		}
	}

	refPath := strings.Split(ref, "#")[0]
	if refPath != "" && refPath != "." && refPath != "./" {
		baseName := filepath.Base(refPath)
		baseName = strings.TrimSuffix(baseName, filepath.Ext(baseName))
		if baseName != "" && baseName != "." && baseName != ".." {
			return r.normalizeComponentName(baseName)
		}
	}

	if schemaMap, ok := componentContent.(map[string]interface{}); ok {
		if title, hasTitle := schemaMap["title"]; hasTitle {
			if titleStr, ok := title.(string); ok && titleStr != "" {
				return r.normalizeComponentName(titleStr)
			}
		}
	}

	return r.generateInlineName(refPath, componentType, componentContent)
}

func (r *ReferenceResolver) generateInlineName(refPath, componentType string, componentContent interface{}) string {
	var suffix string

	if schemaMap, ok := componentContent.(map[string]interface{}); ok {
		if schemaType, hasType := schemaMap["type"]; hasType {
			if typeStr, ok := schemaType.(string); ok {
				suffix = strings.Title(typeStr)
			}
		}
	}

	if suffix == "" && len(componentType) > 1 {
		suffix = strings.Title(componentType[:len(componentType)-1])
	}

	if refPath != "" {
		pathParts := strings.Split(strings.Trim(refPath, "./"), "/")
		var parts []string
		for _, part := range pathParts {
			if part != "" {
				parts = append(parts, strings.TrimSuffix(part, filepath.Ext(part)))
			}
		}
		if len(parts) > 0 {
			return r.normalizeComponentName("Inline_" + strings.Join(parts, "_") + "_" + suffix)
		}
	}

	return r.normalizeComponentName("Inline_" + suffix)
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
		b := builderPool.Get().(*strings.Builder)
		defer func() {
			b.Reset()
			builderPool.Put(b)
		}()
		if strings.HasPrefix(preferredName, "Inline_") {
			baseName := strings.TrimPrefix(preferredName, "Inline_")
			b.Grow(len("Inline_") + len(baseName) + 10)
			b.WriteString("Inline_")
			b.WriteString(baseName)
			b.WriteString(strconv.Itoa(counter))
		} else {
			b.Grow(len(preferredName) + 10)
			b.WriteString(preferredName)
			b.WriteString(strconv.Itoa(counter))
		}
		name = b.String()
	}
}

func (r *ReferenceResolver) hashComponent(component interface{}) string {
	normalized := r.normalizeComponent(component)
	data, err := json.Marshal(normalized)
	if err != nil {
		return ""
	}
	var unmarshaled interface{}
	if err := json.Unmarshal(data, &unmarshaled); err == nil {
		data, _ = json.Marshal(unmarshaled)
	}
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash)
}

func (r *ReferenceResolver) normalizeComponent(component interface{}) interface{} {
	switch v := component.(type) {
	case map[string]interface{}:
		keys := make([]string, 0, len(v))
		for k := range v {
			if k != "description" && k != "example" &&
				k != "title" && k != "deprecated" && k != "externalDocs" && k != "xml" &&
				k != "nullable" && k != "readOnly" && k != "writeOnly" {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		normalized := make(map[string]interface{}, len(keys))
		for _, k := range keys {
			normalized[k] = r.normalizeComponent(v[k])
		}
		return normalized
	case []interface{}:
		normalized := make([]interface{}, len(v))
		for i, val := range v {
			normalized[i] = r.normalizeComponent(val)
		}
		if len(normalized) > 0 {
			if _, ok := normalized[0].(string); ok {
				strSlice := make([]string, len(normalized))
				allStrings := true
				for j, item := range normalized {
					if s, ok := item.(string); ok {
						strSlice[j] = s
					} else {
						allStrings = false
						break
					}
				}
				if allStrings {
					sort.Strings(strSlice)
					result := make([]interface{}, len(strSlice))
					for j, s := range strSlice {
						result[j] = s
					}
					return result
				}
			}
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

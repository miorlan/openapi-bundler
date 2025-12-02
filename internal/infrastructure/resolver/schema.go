package resolver

import (
	"path/filepath"
	"strings"
)

// findSchemaByPath looks up a schema name by file path in all available mappings.
// Returns schema name and true if found, empty string and false otherwise.
func (r *ReferenceResolver) findSchemaByPath(filePath string) (string, bool) {
	// Normalize the path
	var normalizedPath string
	if absPath, err := filepath.Abs(filePath); err == nil {
		normalizedPath = absPath
	} else {
		normalizedPath = filePath
	}

	// Check direct mapping
	if name, exists := r.schemaFileToName[normalizedPath]; exists {
		return name, true
	}

	// Check without extension
	pathWithoutExt := strings.TrimSuffix(normalizedPath, filepath.Ext(normalizedPath))
	if name, exists := r.schemaFileToName[pathWithoutExt]; exists {
		return name, true
	}

	// Try to find by filename in schemas directory
	if !strings.Contains(normalizedPath, "schemas") {
		return "", false
	}

	fileName := filepath.Base(normalizedPath)
	fileNameWithoutExt := strings.TrimSuffix(fileName, filepath.Ext(fileName))
	normalizedFileName := r.normalizeComponentName(fileNameWithoutExt)

	// Check by filename in schemaFileToName
	for path, compName := range r.schemaFileToName {
		baseName := filepath.Base(path)
		if baseName == fileName || baseName == fileNameWithoutExt {
			// Cache the mapping
			r.schemaFileToName[normalizedPath] = compName
			r.schemaFileToName[pathWithoutExt] = compName
			return compName, true
		}
	}

	// Check in rootDoc components
	if components, ok := r.rootDoc["components"].(map[string]interface{}); ok {
		if schemas, ok := components["schemas"].(map[string]interface{}); ok {
			if _, exists := schemas[normalizedFileName]; exists {
				r.schemaFileToName[normalizedPath] = normalizedFileName
				r.schemaFileToName[pathWithoutExt] = normalizedFileName
				return normalizedFileName, true
			}
			for compName := range schemas {
				if r.normalizeComponentName(compName) == normalizedFileName {
					r.schemaFileToName[normalizedPath] = compName
					r.schemaFileToName[pathWithoutExt] = compName
					return compName, true
				}
			}
		}
	}

	// Check in collected components
	if schemas, ok := r.components["schemas"]; ok {
		if _, exists := schemas[normalizedFileName]; exists {
			r.schemaFileToName[normalizedPath] = normalizedFileName
			r.schemaFileToName[pathWithoutExt] = normalizedFileName
			return normalizedFileName, true
		}
		for compName := range schemas {
			if r.normalizeComponentName(compName) == normalizedFileName {
				r.schemaFileToName[normalizedPath] = compName
				r.schemaFileToName[pathWithoutExt] = compName
				return compName, true
			}
		}
	}

	return "", false
}



package infrastructure

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/miorlan/openapi-bundler/internal/domain"
)

func TestReferenceResolver_ResolveAll(t *testing.T) {
	tmpDir := t.TempDir()
	refFile := filepath.Join(tmpDir, "ref.yaml")
	refContent := []byte(`
type: object
properties:
  name:
    type: string
`)

	if err := os.WriteFile(refFile, refContent, 0644); err != nil {
		t.Fatalf("Failed to create ref file: %v", err)
	}

	loader := NewFileLoader()
	parser := NewParser()
	resolver := NewReferenceResolver(loader, parser)

	data := map[string]interface{}{
		"openapi": "3.0.0",
		"components": map[string]interface{}{
			"schemas": map[string]interface{}{
				"User": map[string]interface{}{
					"$ref": refFile,
				},
			},
		},
	}

	ctx := context.Background()
	config := domain.Config{MaxDepth: 10}

	err := resolver.ResolveAll(ctx, data, tmpDir, config)
	if err != nil {
		t.Fatalf("ResolveAll() error = %v", err)
	}
}

func TestReferenceResolver_ResolveAll_InternalRef(t *testing.T) {
	loader := NewFileLoader()
	parser := NewParser()
	resolver := NewReferenceResolver(loader, parser)

	data := map[string]interface{}{
		"openapi": "3.0.0",
		"components": map[string]interface{}{
			"schemas": map[string]interface{}{
				"User": map[string]interface{}{
					"$ref": "#/components/schemas/UserRef",
				},
				"UserRef": map[string]interface{}{
					"type": "object",
				},
			},
		},
	}

	ctx := context.Background()
	config := domain.Config{MaxDepth: 10}

	err := resolver.ResolveAll(ctx, data, "/tmp", config)
	if err != nil {
		t.Fatalf("ResolveAll() error = %v", err)
	}
}

func TestReferenceResolver_ResolveAll_MaxDepth(t *testing.T) {
	tmpDir := t.TempDir()
	refFile := filepath.Join(tmpDir, "ref.yaml")
	refContent := []byte(`
type: object
properties:
  name:
    type: string
`)

	if err := os.WriteFile(refFile, refContent, 0644); err != nil {
		t.Fatalf("Failed to create ref file: %v", err)
	}

	loader := NewFileLoader()
	parser := NewParser()
	resolver := NewReferenceResolver(loader, parser)

	data := map[string]interface{}{
		"openapi": "3.0.0",
		"components": map[string]interface{}{
			"schemas": map[string]interface{}{
				"User": map[string]interface{}{
					"$ref": refFile,
				},
			},
		},
	}

	ctx := context.Background()
	config := domain.Config{MaxDepth: 1}

	err := resolver.ResolveAll(ctx, data, tmpDir, config)
	if err != nil {
		t.Fatalf("ResolveAll() error = %v", err)
	}
}

func TestReferenceResolver_ResolveAll_Array(t *testing.T) {
	tmpDir := t.TempDir()
	refFile := filepath.Join(tmpDir, "ref.yaml")
	refContent := []byte(`
type: array
items:
  type: string
`)

	if err := os.WriteFile(refFile, refContent, 0644); err != nil {
		t.Fatalf("Failed to create ref file: %v", err)
	}

	loader := NewFileLoader()
	parser := NewParser()
	resolver := NewReferenceResolver(loader, parser)

	data := map[string]interface{}{
		"openapi": "3.0.0",
		"components": map[string]interface{}{
			"schemas": map[string]interface{}{
				"User": map[string]interface{}{
					"$ref": refFile,
				},
			},
		},
	}

	ctx := context.Background()
	config := domain.Config{MaxDepth: 10}

	err := resolver.ResolveAll(ctx, data, tmpDir, config)
	if err != nil {
		t.Fatalf("ResolveAll() error = %v", err)
	}

	schemas := data["components"].(map[string]interface{})["schemas"].(map[string]interface{})
	user := schemas["User"].(map[string]interface{})
	if refVal, hasRef := user["$ref"]; hasRef {
		if refStr, ok := refVal.(string); !ok {
			t.Errorf("$ref should be a string, got %T", refVal)
		} else if !strings.HasPrefix(refStr, "#/components/schemas/") {
			t.Errorf("$ref should be an internal reference, got %s", refStr)
		}
	}
}

func TestReferenceResolver_Resolve_Fragment(t *testing.T) {
	tmpDir := t.TempDir()
	refFile := filepath.Join(tmpDir, "ref.yaml")
	refContent := []byte(`
openapi: 3.0.0
components:
  schemas:
    User:
      type: object
      properties:
        name:
          type: string
`)

	if err := os.WriteFile(refFile, refContent, 0644); err != nil {
		t.Fatalf("Failed to create ref file: %v", err)
	}

	loader := NewFileLoader()
	parser := NewParser()
	resolver := NewReferenceResolver(loader, parser)

	data := map[string]interface{}{
		"openapi": "3.0.0",
		"components": map[string]interface{}{
			"schemas": map[string]interface{}{
				"UserRef": map[string]interface{}{
					"$ref": refFile + "#/components/schemas/User",
				},
			},
		},
	}

	ctx := context.Background()
	config := domain.Config{MaxDepth: 10}

	err := resolver.ResolveAll(ctx, data, tmpDir, config)
	if err != nil {
		t.Fatalf("ResolveAll() error = %v", err)
	}

	schemas := data["components"].(map[string]interface{})["schemas"].(map[string]interface{})
	
	// После "поднятия" $ref, UserRef должен содержать реальное содержимое, а не ссылку
	userRef, exists := schemas["UserRef"]
	if !exists {
		t.Error("UserRef should exist in components/schemas")
	}
	
	userRefMap, ok := userRef.(map[string]interface{})
	if !ok {
		t.Fatalf("UserRef should be a map, got %T", userRef)
	}
	
	// После "поднятия" должно быть реальное содержимое, а не $ref
	if _, hasRef := userRefMap["$ref"]; hasRef {
		t.Error("UserRef should not contain $ref after lifting - it should contain actual schema content")
	}
	
	// Проверяем, что содержимое соответствует схеме User
	if userType, hasType := userRefMap["type"]; !hasType || userType != "object" {
		t.Error("UserRef should contain the actual schema content (type: object) after lifting")
	}
	
	if properties, hasProps := userRefMap["properties"]; !hasProps {
		t.Error("UserRef should contain properties after lifting")
	} else {
		propsMap, ok := properties.(map[string]interface{})
		if !ok {
			t.Errorf("properties should be a map, got %T", properties)
		} else {
			if _, hasName := propsMap["name"]; !hasName {
				t.Error("UserRef should contain 'name' property after lifting")
			}
		}
	}

	if _, exists := schemas["User"]; !exists {
		t.Error("User schema should be added to components/schemas")
	}
}

func TestReferenceResolver_ResolveAll_NoInlineNestedObjects(t *testing.T) {
	tmpDir := t.TempDir()
	externalFile := filepath.Join(tmpDir, "External.yaml")
	externalContent := []byte(`
type: object
properties:
  name:
    type: string
`)

	if err := os.WriteFile(externalFile, externalContent, 0644); err != nil {
		t.Fatalf("Failed to create external file: %v", err)
	}

	loader := NewFileLoader()
	parser := NewParser()
	resolver := NewReferenceResolver(loader, parser)

	data := map[string]interface{}{
		"openapi": "3.0.0",
		"components": map[string]interface{}{
			"schemas": map[string]interface{}{
				"Parent": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"child": map[string]interface{}{
							"$ref": externalFile,
						},
					},
				},
			},
		},
	}

	ctx := context.Background()
	config := domain.Config{MaxDepth: 10}

	err := resolver.ResolveAll(ctx, data, tmpDir, config)
	if err != nil {
		t.Fatalf("ResolveAll() error = %v", err)
	}

	schemas := data["components"].(map[string]interface{})["schemas"].(map[string]interface{})
	parent := schemas["Parent"].(map[string]interface{})
	properties := parent["properties"].(map[string]interface{})
	child := properties["child"].(map[string]interface{})
	
	ref, hasRef := child["$ref"]
	if !hasRef {
		t.Error("child should contain $ref")
	}

	refStr, ok := ref.(string)
	if !ok {
		t.Fatalf("$ref should be a string, got %T", ref)
	}

	if !strings.HasPrefix(refStr, "#/components/schemas/") {
		t.Errorf("$ref should be an internal reference, got %s", refStr)
	}
}

func TestReferenceResolver_ExpandPathsSection(t *testing.T) {
	tmpDir := t.TempDir()
	pathsDir := filepath.Join(tmpDir, "paths")
	if err := os.MkdirAll(pathsDir, 0755); err != nil {
		t.Fatalf("Failed to create paths directory: %v", err)
	}

	pathsFile := filepath.Join(pathsDir, "_index.yaml")
	pathsContent := []byte(`/api/v1/users:
  get:
    summary: Get users
    responses:
      '200':
        description: OK
`)

	if err := os.WriteFile(pathsFile, pathsContent, 0644); err != nil {
		t.Fatalf("Failed to create paths file: %v", err)
	}

	loader := NewFileLoader()
	parser := NewParser()
	resolver := NewReferenceResolver(loader, parser)

	data := map[string]interface{}{
		"openapi": "3.0.0",
		"paths": map[string]interface{}{
			"$ref": "./paths/_index.yaml",
		},
	}

	ctx := context.Background()
	config := domain.Config{MaxDepth: 10}

	err := resolver.ResolveAll(ctx, data, tmpDir, config)
	if err != nil {
		t.Fatalf("ResolveAll() error = %v", err)
	}

	paths := data["paths"].(map[string]interface{})

	if _, exists := paths["/api/v1/users"]; !exists {
		t.Error("paths should contain /api/v1/users")
	}
}

func TestReferenceResolver_ExpandPathsSection_StringRef(t *testing.T) {
	tmpDir := t.TempDir()
	pathsDir := filepath.Join(tmpDir, "paths")
	if err := os.MkdirAll(pathsDir, 0755); err != nil {
		t.Fatalf("Failed to create paths directory: %v", err)
	}

	pathsFile := filepath.Join(pathsDir, "_index.yaml")
	pathsContent := []byte(`/api/v1/users:
  get:
    summary: Get users
    responses:
      '200':
        description: OK
`)

	if err := os.WriteFile(pathsFile, pathsContent, 0644); err != nil {
		t.Fatalf("Failed to create paths file: %v", err)
	}

	loader := NewFileLoader()
	parser := NewParser()
	resolver := NewReferenceResolver(loader, parser)

	data := map[string]interface{}{
		"openapi": "3.0.0",
		"paths": "./paths/_index.yaml",
	}

	ctx := context.Background()
	config := domain.Config{MaxDepth: 10}

	err := resolver.ResolveAll(ctx, data, tmpDir, config)
	if err != nil {
		t.Fatalf("ResolveAll() error = %v", err)
	}

	paths := data["paths"].(map[string]interface{})

	if _, exists := paths["/api/v1/users"]; !exists {
		t.Error("paths should contain /api/v1/users")
	}
}

func TestReferenceResolver_ExpandComponentsParameters(t *testing.T) {
	tmpDir := t.TempDir()
	paramsDir := filepath.Join(tmpDir, "parameters")
	if err := os.MkdirAll(paramsDir, 0755); err != nil {
		t.Fatalf("Failed to create parameters directory: %v", err)
	}

	paramsFile := filepath.Join(paramsDir, "_index.yaml")
	paramsContent := []byte(`X-App-Version:
  name: X-App-Version
  in: header
  schema:
    type: string
`)

	if err := os.WriteFile(paramsFile, paramsContent, 0644); err != nil {
		t.Fatalf("Failed to create parameters file: %v", err)
	}

	loader := NewFileLoader()
	parser := NewParser()
	resolver := NewReferenceResolver(loader, parser)

	data := map[string]interface{}{
		"openapi": "3.0.0",
		"components": map[string]interface{}{
			"parameters": "./parameters/_index.yaml",
		},
	}

	ctx := context.Background()
	config := domain.Config{MaxDepth: 10}

	err := resolver.ResolveAll(ctx, data, tmpDir, config)
	if err != nil {
		t.Fatalf("ResolveAll() error = %v", err)
	}

	components := data["components"].(map[string]interface{})
	parameters := components["parameters"].(map[string]interface{})

	if _, exists := parameters["X-App-Version"]; !exists {
		t.Error("parameters should contain X-App-Version")
	}
}

func TestReferenceResolver_ExpandComponentsSchemas(t *testing.T) {
	tmpDir := t.TempDir()
	schemasDir := filepath.Join(tmpDir, "schemas")
	if err := os.MkdirAll(schemasDir, 0755); err != nil {
		t.Fatalf("Failed to create schemas directory: %v", err)
	}

	schemasFile := filepath.Join(schemasDir, "_index.yaml")
	schemasContent := []byte(`User:
  type: object
  properties:
    name:
      type: string
`)

	if err := os.WriteFile(schemasFile, schemasContent, 0644); err != nil {
		t.Fatalf("Failed to create schemas file: %v", err)
	}

	loader := NewFileLoader()
	parser := NewParser()
	resolver := NewReferenceResolver(loader, parser)

	data := map[string]interface{}{
		"openapi": "3.0.0",
		"components": map[string]interface{}{
			"schemas": "./schemas/_index.yaml",
		},
	}

	ctx := context.Background()
	config := domain.Config{MaxDepth: 10}

	err := resolver.ResolveAll(ctx, data, tmpDir, config)
	if err != nil {
		t.Fatalf("ResolveAll() error = %v", err)
	}

	components := data["components"].(map[string]interface{})
	schemas := components["schemas"].(map[string]interface{})

	if _, exists := schemas["User"]; !exists {
		t.Error("schemas should contain User")
	}
}

func TestReferenceResolver_ExpandMultipleSections(t *testing.T) {
	tmpDir := t.TempDir()
	pathsDir := filepath.Join(tmpDir, "paths")
	paramsDir := filepath.Join(tmpDir, "parameters")
	if err := os.MkdirAll(pathsDir, 0755); err != nil {
		t.Fatalf("Failed to create paths directory: %v", err)
	}
	if err := os.MkdirAll(paramsDir, 0755); err != nil {
		t.Fatalf("Failed to create parameters directory: %v", err)
	}

	pathsFile := filepath.Join(pathsDir, "_index.yaml")
	pathsContent := []byte(`/api/v1/users:
  get:
    summary: Get users
    responses:
      '200':
        description: OK
`)

	paramsFile := filepath.Join(paramsDir, "_index.yaml")
	paramsContent := []byte(`X-App-Version:
  name: X-App-Version
  in: header
  schema:
    type: string
`)

	if err := os.WriteFile(pathsFile, pathsContent, 0644); err != nil {
		t.Fatalf("Failed to create paths file: %v", err)
	}
	if err := os.WriteFile(paramsFile, paramsContent, 0644); err != nil {
		t.Fatalf("Failed to create parameters file: %v", err)
	}

	loader := NewFileLoader()
	parser := NewParser()
	resolver := NewReferenceResolver(loader, parser)

	data := map[string]interface{}{
		"openapi": "3.0.0",
		"paths": "./paths/_index.yaml",
		"components": map[string]interface{}{
			"parameters": "./parameters/_index.yaml",
		},
	}

	ctx := context.Background()
	config := domain.Config{MaxDepth: 10}

	err := resolver.ResolveAll(ctx, data, tmpDir, config)
	if err != nil {
		t.Fatalf("ResolveAll() error = %v", err)
	}

	paths := data["paths"].(map[string]interface{})
	if _, exists := paths["/api/v1/users"]; !exists {
		t.Error("paths should contain /api/v1/users")
	}

	components := data["components"].(map[string]interface{})
	parameters := components["parameters"].(map[string]interface{})
	if _, exists := parameters["X-App-Version"]; !exists {
		t.Error("parameters should contain X-App-Version")
	}
}

func TestReferenceResolver_ExpandSectionWithNestedRefs(t *testing.T) {
	tmpDir := t.TempDir()
	schemasDir := filepath.Join(tmpDir, "schemas")
	if err := os.MkdirAll(schemasDir, 0755); err != nil {
		t.Fatalf("Failed to create schemas directory: %v", err)
	}

	userFile := filepath.Join(schemasDir, "User.yaml")
	userContent := []byte(`type: object
properties:
  name:
    type: string
`)

	addressFile := filepath.Join(schemasDir, "Address.yaml")
	addressContent := []byte(`type: object
properties:
  street:
    type: string
`)

	if err := os.WriteFile(userFile, userContent, 0644); err != nil {
		t.Fatalf("Failed to create user file: %v", err)
	}
	if err := os.WriteFile(addressFile, addressContent, 0644); err != nil {
		t.Fatalf("Failed to create address file: %v", err)
	}

	loader := NewFileLoader()
	parser := NewParser()
	resolver := NewReferenceResolver(loader, parser)

	data := map[string]interface{}{
		"openapi": "3.0.0",
		"components": map[string]interface{}{
			"schemas": map[string]interface{}{
				"Person": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"user": map[string]interface{}{
							"$ref": "./schemas/User.yaml",
						},
						"address": map[string]interface{}{
							"$ref": "./schemas/Address.yaml",
						},
					},
				},
			},
		},
	}

	ctx := context.Background()
	config := domain.Config{MaxDepth: 10}

	err := resolver.ResolveAll(ctx, data, tmpDir, config)
	if err != nil {
		t.Fatalf("ResolveAll() error = %v", err)
	}

	components := data["components"].(map[string]interface{})
	schemas := components["schemas"].(map[string]interface{})
	person := schemas["Person"].(map[string]interface{})
	properties := person["properties"].(map[string]interface{})
	user := properties["user"].(map[string]interface{})
	
	ref, hasRef := user["$ref"]
	if !hasRef {
		t.Error("user should contain $ref")
	}

	refStr, ok := ref.(string)
	if !ok {
		t.Fatalf("$ref should be a string, got %T", ref)
	}

	if !strings.HasPrefix(refStr, "#/components/schemas/") {
		t.Errorf("$ref should be an internal reference, got %s", refStr)
	}
}

func TestReferenceResolver_ExpandSectionWithRelativeRefs(t *testing.T) {
	tmpDir := t.TempDir()
	apiDir := filepath.Join(tmpDir, "api")
	v1Dir := filepath.Join(apiDir, "v1")
	if err := os.MkdirAll(v1Dir, 0755); err != nil {
		t.Fatalf("Failed to create v1 directory: %v", err)
	}

	pathsFile := filepath.Join(v1Dir, "paths.yaml")
	pathsContent := []byte(`/api/v1/table-registry:
  get:
    summary: Get table registry
    responses:
      '200':
        description: OK
`)

	if err := os.WriteFile(pathsFile, pathsContent, 0644); err != nil {
		t.Fatalf("Failed to create paths file: %v", err)
	}

	loader := NewFileLoader()
	parser := NewParser()
	resolver := NewReferenceResolver(loader, parser)

	data := map[string]interface{}{
		"openapi": "3.0.0",
		"paths": map[string]interface{}{
			"$ref": "./api/v1/paths.yaml",
		},
	}

	ctx := context.Background()
	config := domain.Config{MaxDepth: 10}

	err := resolver.ResolveAll(ctx, data, tmpDir, config)
	if err != nil {
		t.Fatalf("ResolveAll() error = %v", err)
	}

	paths := data["paths"].(map[string]interface{})
	tableRegistryPath := paths["/api/v1/table-registry"].(map[string]interface{})
	if _, exists := tableRegistryPath["get"]; !exists {
		t.Error("table-registry path should contain get method")
	}
}

func TestReferenceResolver_ResolveAll_AllOfWithExternalRef(t *testing.T) {
	tmpDir := t.TempDir()
	baseFile := filepath.Join(tmpDir, "Base.yaml")
	baseContent := []byte(`type: object
properties:
  id:
    type: integer
`)

	if err := os.WriteFile(baseFile, baseContent, 0644); err != nil {
		t.Fatalf("Failed to create base file: %v", err)
	}

	loader := NewFileLoader()
	parser := NewParser()
	resolver := NewReferenceResolver(loader, parser)

	data := map[string]interface{}{
		"openapi": "3.0.0",
		"components": map[string]interface{}{
			"schemas": map[string]interface{}{
				"User": map[string]interface{}{
					"allOf": []interface{}{
						map[string]interface{}{
							"$ref": baseFile,
						},
						map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"name": map[string]interface{}{
									"type": "string",
								},
							},
						},
					},
				},
			},
		},
	}

	ctx := context.Background()
	config := domain.Config{MaxDepth: 10}

	err := resolver.ResolveAll(ctx, data, tmpDir, config)
	if err != nil {
		t.Fatalf("ResolveAll() error = %v", err)
	}

	schemas := data["components"].(map[string]interface{})["schemas"].(map[string]interface{})
	user := schemas["User"].(map[string]interface{})
	allOf := user["allOf"].([]interface{})
	firstItem := allOf[0].(map[string]interface{})
	
	ref, hasRef := firstItem["$ref"]
	if !hasRef {
		t.Error("first allOf item should contain $ref")
	}

	refStr, ok := ref.(string)
	if !ok {
		t.Fatalf("$ref should be a string, got %T", ref)
	}

	if !strings.HasPrefix(refStr, "#/components/schemas/") {
		t.Errorf("$ref should be an internal reference, got %s", refStr)
	}
}

func TestReferenceResolver_ResolveAll_ItemsWithExternalRef(t *testing.T) {
	tmpDir := t.TempDir()
	contactFile := filepath.Join(tmpDir, "Contact.yaml")
	contactContent := []byte(`type: object
properties:
  email:
    type: string
`)

	if err := os.WriteFile(contactFile, contactContent, 0644); err != nil {
		t.Fatalf("Failed to create contact file: %v", err)
	}

	loader := NewFileLoader()
	parser := NewParser()
	resolver := NewReferenceResolver(loader, parser)

	data := map[string]interface{}{
		"openapi": "3.0.0",
		"components": map[string]interface{}{
			"schemas": map[string]interface{}{
				"User": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"contacts": map[string]interface{}{
							"type": "array",
							"items": map[string]interface{}{
								"$ref": contactFile,
							},
						},
					},
				},
			},
		},
	}

	ctx := context.Background()
	config := domain.Config{MaxDepth: 10}

	err := resolver.ResolveAll(ctx, data, tmpDir, config)
	if err != nil {
		t.Fatalf("ResolveAll() error = %v", err)
	}

	schemas := data["components"].(map[string]interface{})["schemas"].(map[string]interface{})
	user := schemas["User"].(map[string]interface{})
	properties := user["properties"].(map[string]interface{})
	contacts := properties["contacts"].(map[string]interface{})
	items := contacts["items"].(map[string]interface{})
	
	ref, hasRef := items["$ref"]
	if !hasRef {
		t.Error("items should contain $ref")
	}

	refStr, ok := ref.(string)
	if !ok {
		t.Fatalf("$ref should be a string, got %T", ref)
	}

	if !strings.HasPrefix(refStr, "#/components/schemas/") {
		t.Errorf("$ref should be an internal reference, got %s", refStr)
	}
}

func TestReferenceResolver_ResolveAll_ComponentsWithParentDirRef(t *testing.T) {
	tmpDir := t.TempDir()
	rootFile := filepath.Join(tmpDir, "openapi.yaml")
	schemasDir := filepath.Join(tmpDir, "schemas")
	if err := os.MkdirAll(schemasDir, 0755); err != nil {
		t.Fatalf("Failed to create schemas directory: %v", err)
	}
	
	externalFile := filepath.Join(schemasDir, "Name.yaml")
	externalContent := []byte(`type: string
description: Name field
`)
	if err := os.WriteFile(externalFile, externalContent, 0644); err != nil {
		t.Fatalf("Failed to create external file: %v", err)
	}

	loader := NewFileLoader()
	parser := NewParser()
	resolver := NewReferenceResolver(loader, parser)

	data := map[string]interface{}{
		"openapi": "3.0.0",
		"components": map[string]interface{}{
			"schemas": map[string]interface{}{
				"EmployeeShortInfo": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"name": map[string]interface{}{
							"$ref": "./schemas/Name.yaml",
						},
					},
				},
			},
		},
	}

	ctx := context.Background()
	config := domain.Config{MaxDepth: 10}

	basePath := filepath.Dir(rootFile)
	err := resolver.ResolveAll(ctx, data, basePath, config)
	if err != nil {
		t.Fatalf("ResolveAll() error = %v", err)
	}

	schemas := data["components"].(map[string]interface{})["schemas"].(map[string]interface{})
	employeeShortInfo := schemas["EmployeeShortInfo"].(map[string]interface{})
	properties := employeeShortInfo["properties"].(map[string]interface{})
	name := properties["name"].(map[string]interface{})
	
	ref, hasRef := name["$ref"]
	if !hasRef {
		t.Error("name should contain $ref")
	}

	refStr, ok := ref.(string)
	if !ok {
		t.Fatalf("$ref should be a string, got %T", ref)
	}

	if !strings.HasPrefix(refStr, "#/components/schemas/") {
		t.Errorf("$ref should be an internal reference, got %s", refStr)
	}
}

// TestReferenceResolver_ComponentNames_RealNames проверяет, что компоненты получают реальные имена,
// а не автоматически сгенерированные типа SchemaN
func TestReferenceResolver_ComponentNames_RealNames(t *testing.T) {
	tmpDir := t.TempDir()
	schemasDir := filepath.Join(tmpDir, "schemas")
	if err := os.MkdirAll(schemasDir, 0755); err != nil {
		t.Fatalf("Failed to create schemas directory: %v", err)
	}

	// Создаём файл с компонентом Error
	errorFile := filepath.Join(schemasDir, "Error.yaml")
	errorContent := []byte(`type: object
properties:
  message:
    type: string
  code:
    type: integer
`)
	if err := os.WriteFile(errorFile, errorContent, 0644); err != nil {
		t.Fatalf("Failed to create error file: %v", err)
	}

	// Создаём файл с компонентом ChangePasswordRequest
	changePasswordFile := filepath.Join(schemasDir, "ChangePasswordRequest.yaml")
	changePasswordContent := []byte(`type: object
properties:
  oldPassword:
    type: string
  newPassword:
    type: string
`)
	if err := os.WriteFile(changePasswordFile, changePasswordContent, 0644); err != nil {
		t.Fatalf("Failed to create change password file: %v", err)
	}

	// Создаём файл с компонентом в components/schemas
	requestGuestsFile := filepath.Join(schemasDir, "RequestGuests.yaml")
	requestGuestsContent := []byte(`openapi: 3.0.0
components:
  schemas:
    RequestGuests:
      type: object
      properties:
        guests:
          type: array
`)
	if err := os.WriteFile(requestGuestsFile, requestGuestsContent, 0644); err != nil {
		t.Fatalf("Failed to create request guests file: %v", err)
	}

	loader := NewFileLoader()
	parser := NewParser()
	resolver := NewReferenceResolver(loader, parser)

	data := map[string]interface{}{
		"openapi": "3.0.0",
		"paths": map[string]interface{}{
			"/api/v1/profile/change-password": map[string]interface{}{
				"post": map[string]interface{}{
					"requestBody": map[string]interface{}{
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"$ref": "./schemas/ChangePasswordRequest.yaml",
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"400": map[string]interface{}{
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "./schemas/Error.yaml",
									},
								},
							},
						},
						"418": map[string]interface{}{
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "./schemas/Error.yaml",
									},
								},
							},
						},
					},
				},
			},
		},
		"components": map[string]interface{}{
			"schemas": map[string]interface{}{
				"RequestGuests": map[string]interface{}{
					"$ref": "./schemas/RequestGuests.yaml#/components/schemas/RequestGuests",
				},
			},
		},
	}

	ctx := context.Background()
	config := domain.Config{MaxDepth: 10}

	basePath := tmpDir
	err := resolver.ResolveAll(ctx, data, basePath, config)
	if err != nil {
		t.Fatalf("ResolveAll() error = %v", err)
	}

	// Проверяем, что компоненты имеют правильные имена
	schemas := data["components"].(map[string]interface{})["schemas"].(map[string]interface{})

	// Проверяем Error - должно быть имя из файла
	if _, exists := schemas["Error"]; !exists {
		t.Error("Error schema should exist with name 'Error'")
	}

	// Проверяем ChangePasswordRequest - должно быть имя из файла
	if _, exists := schemas["ChangePasswordRequest"]; !exists {
		t.Error("ChangePasswordRequest schema should exist with name 'ChangePasswordRequest'")
	}

	// Проверяем RequestGuests - должно быть имя из фрагмента
	if _, exists := schemas["RequestGuests"]; !exists {
		t.Error("RequestGuests schema should exist with name 'RequestGuests'")
	}

	// Проверяем, что НЕТ имён типа SchemaN
	for name := range schemas {
		if strings.HasPrefix(name, "Schema") {
			// Проверяем, что это не SchemaN (где N - число)
			if len(name) > 6 {
				rest := name[6:]
				isNumber := true
				for _, r := range rest {
					if r < '0' || r > '9' {
						isNumber = false
						break
					}
				}
				if isNumber {
					t.Errorf("Found auto-generated schema name: %s. Should use real names like Error, ChangePasswordRequest", name)
				}
			}
		}
	}

	// Проверяем, что $ref указывают на правильные имена
	paths := data["paths"].(map[string]interface{})
	path := paths["/api/v1/profile/change-password"].(map[string]interface{})
	post := path["post"].(map[string]interface{})
	
	// Проверяем requestBody
	requestBody := post["requestBody"].(map[string]interface{})
	content := requestBody["content"].(map[string]interface{})
	appJson := content["application/json"].(map[string]interface{})
	schema := appJson["schema"].(map[string]interface{})
	ref, hasRef := schema["$ref"]
	if !hasRef {
		t.Error("schema should contain $ref")
	}
	refStr, ok := ref.(string)
	if !ok {
		t.Fatalf("$ref should be a string, got %T", ref)
	}
	if refStr != "#/components/schemas/ChangePasswordRequest" {
		t.Errorf("$ref should be '#/components/schemas/ChangePasswordRequest', got '%s'", refStr)
	}

	// Проверяем responses
	responses := post["responses"].(map[string]interface{})
	
	// Проверяем 400 response
	response400 := responses["400"].(map[string]interface{})
	content400 := response400["content"].(map[string]interface{})
	appJson400 := content400["application/json"].(map[string]interface{})
	schema400 := appJson400["schema"].(map[string]interface{})
	ref400, hasRef400 := schema400["$ref"]
	if !hasRef400 {
		t.Error("schema in 400 response should contain $ref")
	}
	refStr400, ok := ref400.(string)
	if !ok {
		t.Fatalf("$ref should be a string, got %T", ref400)
	}
	if refStr400 != "#/components/schemas/Error" {
		t.Errorf("$ref should be '#/components/schemas/Error', got '%s'", refStr400)
	}

	// Проверяем 418 response
	response418 := responses["418"].(map[string]interface{})
	content418 := response418["content"].(map[string]interface{})
	appJson418 := content418["application/json"].(map[string]interface{})
	schema418 := appJson418["schema"].(map[string]interface{})
	ref418, hasRef418 := schema418["$ref"]
	if !hasRef418 {
		t.Error("schema in 418 response should contain $ref")
	}
	refStr418, ok := ref418.(string)
	if !ok {
		t.Fatalf("$ref should be a string, got %T", ref418)
	}
	if refStr418 != "#/components/schemas/Error" {
		t.Errorf("$ref should be '#/components/schemas/Error', got '%s'", refStr418)
	}

	// Проверяем RequestGuests
	requestGuests := schemas["RequestGuests"].(map[string]interface{})
	if requestGuests == nil {
		t.Error("RequestGuests should exist")
	}
}

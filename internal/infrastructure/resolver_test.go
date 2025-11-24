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
					"$ref": "ref.yaml",
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

func TestReferenceResolver_ResolveAll_Array(t *testing.T) {
	loader := NewFileLoader()
	parser := NewParser()
	resolver := NewReferenceResolver(loader, parser)

	data := map[string]interface{}{
		"openapi": "3.0.0",
		"paths": []interface{}{
			map[string]interface{}{
				"$ref": "#/components/schemas/User",
			},
		},
		"components": map[string]interface{}{
			"schemas": map[string]interface{}{
				"User": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"name": map[string]interface{}{
							"type": "string",
						},
					},
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

	paths := data["paths"].([]interface{})
	pathItem := paths[0].(map[string]interface{})
	if ref, ok := pathItem["$ref"]; !ok {
		t.Error("path item should contain $ref")
	} else if refStr, ok := ref.(string); !ok {
		t.Errorf("$ref should be a string, got %T", ref)
	} else if !strings.HasPrefix(refStr, "#/components/schemas/") {
		t.Errorf("$ref should be an internal reference, got %s", refStr)
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
	userRef := schemas["UserRef"].(map[string]interface{})
	ref, hasRef := userRef["$ref"]
	if !hasRef {
		t.Error("UserRef should contain $ref")
	}

	refStr, ok := ref.(string)
	if !ok {
		t.Fatalf("$ref should be a string, got %T", ref)
	}

	if !strings.HasPrefix(refStr, "#/components/schemas/") {
		t.Errorf("$ref should be an internal reference, got %s", refStr)
	}

	if _, exists := schemas["User"]; !exists {
		t.Error("User schema should be added to components/schemas")
	}
}

func TestReferenceResolver_ResolveAll_NoInlineNestedObjects(t *testing.T) {
	tmpDir := t.TempDir()
	externalFile := filepath.Join(tmpDir, "External.yaml")
	externalContent := []byte(`type: object
properties:
  value:
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
							"$ref": "./External.yaml",
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
		t.Error("child should contain $ref, not be inlined")
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

	paths, ok := data["paths"].(map[string]interface{})
	if !ok {
		t.Fatalf("paths should be a map, got %T", data["paths"])
	}

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

	paths, ok := data["paths"].(map[string]interface{})
	if !ok {
		t.Fatalf("paths should be a map, got %T", data["paths"])
	}

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
  in: header
  name: X-App-Version
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
			"parameters": map[string]interface{}{
				"$ref": "./parameters/_index.yaml",
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
`)
	if err := os.WriteFile(pathsFile, pathsContent, 0644); err != nil {
		t.Fatalf("Failed to create paths file: %v", err)
	}

	paramsFile := filepath.Join(paramsDir, "_index.yaml")
	paramsContent := []byte(`X-App-Version:
  in: header
  name: X-App-Version
`)
	if err := os.WriteFile(paramsFile, paramsContent, 0644); err != nil {
		t.Fatalf("Failed to create parameters file: %v", err)
	}

	loader := NewFileLoader()
	parser := NewParser()
	resolver := NewReferenceResolver(loader, parser)

	data := map[string]interface{}{
		"openapi": "3.0.0",
		"paths": map[string]interface{}{
			"$ref": "./paths/_index.yaml",
		},
		"components": map[string]interface{}{
			"parameters": map[string]interface{}{
				"$ref": "./parameters/_index.yaml",
			},
		},
	}

	ctx := context.Background()
	config := domain.Config{MaxDepth: 10}

	err := resolver.ResolveAll(ctx, data, tmpDir, config)
	if err != nil {
		t.Fatalf("ResolveAll() error = %v", err)
	}

	paths, ok := data["paths"].(map[string]interface{})
	if !ok {
		t.Fatalf("paths should be a map, got %T", data["paths"])
	}
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
	pathsDir := filepath.Join(tmpDir, "paths")
	if err := os.MkdirAll(pathsDir, 0755); err != nil {
		t.Fatalf("Failed to create paths directory: %v", err)
	}

	externalFile := filepath.Join(pathsDir, "User.yaml")
	externalContent := []byte(`type: object
properties:
  name:
    type: string
`)
	if err := os.WriteFile(externalFile, externalContent, 0644); err != nil {
		t.Fatalf("Failed to create external file: %v", err)
	}

	pathsFile := filepath.Join(pathsDir, "_index.yaml")
	pathsContent := []byte(`/api/v1/users:
  get:
    summary: Get users
    responses:
      '200':
        content:
          application/json:
            schema:
              $ref: ./User.yaml
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
	usersPath := paths["/api/v1/users"].(map[string]interface{})
	get := usersPath["get"].(map[string]interface{})
	responses := get["responses"].(map[string]interface{})
	response200 := responses["200"].(map[string]interface{})
	content := response200["content"].(map[string]interface{})
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

	if !strings.HasPrefix(refStr, "#/components/schemas/") {
		t.Errorf("$ref should be an internal reference, got %s", refStr)
	}
}

func TestReferenceResolver_ExpandSectionWithRelativeRefs(t *testing.T) {
	tmpDir := t.TempDir()
	pathsDir := filepath.Join(tmpDir, "paths")
	if err := os.MkdirAll(pathsDir, 0755); err != nil {
		t.Fatalf("Failed to create paths directory: %v", err)
	}

	tableRegistryFile := filepath.Join(pathsDir, "tableRegistry.yaml")
	tableRegistryContent := []byte(`get:
  summary: Get table registry
  responses:
    '200':
      description: OK
`)
	if err := os.WriteFile(tableRegistryFile, tableRegistryContent, 0644); err != nil {
		t.Fatalf("Failed to create tableRegistry file: %v", err)
	}

	pathsFile := filepath.Join(pathsDir, "_index.yaml")
	pathsContent := []byte(`/api/v1/table-registry:
  $ref: ./tableRegistry.yaml
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
	tableRegistryPath := paths["/api/v1/table-registry"].(map[string]interface{})
	if _, exists := tableRegistryPath["get"]; !exists {
		t.Error("table-registry path should contain get method")
	}
}

func TestReferenceResolver_ResolveAll_AllOfWithExternalRef(t *testing.T) {
	tmpDir := t.TempDir()
	
	externalFile := filepath.Join(tmpDir, "Base.yaml")
	externalContent := []byte(`type: object
properties:
  id:
    type: integer
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
				"User": map[string]interface{}{
					"allOf": []interface{}{
						map[string]interface{}{
							"$ref": "./Base.yaml",
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
	
	if len(allOf) != 2 {
		t.Fatalf("allOf should have 2 items, got %d", len(allOf))
	}

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
	
	externalFile := filepath.Join(tmpDir, "GuestContact.yaml")
	externalContent := []byte(`type: object
properties:
  type_id:
    type: integer
  value:
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
				"AnonimGuest": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"contacts": map[string]interface{}{
							"type": "array",
							"items": map[string]interface{}{
								"$ref": "./GuestContact.yaml",
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
	anonimGuest := schemas["AnonimGuest"].(map[string]interface{})
	properties := anonimGuest["properties"].(map[string]interface{})
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

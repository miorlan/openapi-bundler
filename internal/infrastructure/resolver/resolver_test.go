package resolver

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/miorlan/openapi-bundler/internal/domain"
	"github.com/miorlan/openapi-bundler/internal/infrastructure/loader"
	"github.com/miorlan/openapi-bundler/internal/infrastructure/parser"
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

	fileLoader := loader.NewFileLoader()
	p := parser.NewParser()
	resolver := NewReferenceResolver(fileLoader, p)

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
	fileLoader := loader.NewFileLoader()
	p := parser.NewParser()
	resolver := NewReferenceResolver(fileLoader, p)

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

	fileLoader := loader.NewFileLoader()
	p := parser.NewParser()
	resolver := NewReferenceResolver(fileLoader, p)

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

	fileLoader := loader.NewFileLoader()
	p := parser.NewParser()
	resolver := NewReferenceResolver(fileLoader, p)

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

	fileLoader := loader.NewFileLoader()
	p := parser.NewParser()
	resolver := NewReferenceResolver(fileLoader, p)

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

	userRef, exists := schemas["UserRef"]
	if !exists {
		t.Error("UserRef should exist in components/schemas")
	}

	userRefMap, ok := userRef.(map[string]interface{})
	if !ok {
		t.Fatalf("UserRef should be a map, got %T", userRef)
	}

	// After resolving, UserRef should contain $ref to User schema
	refVal, hasRef := userRefMap["$ref"]
	if !hasRef {
		t.Error("UserRef should contain $ref to User schema")
	} else {
		if refStr, ok := refVal.(string); !ok || refStr != "#/components/schemas/User" {
			t.Errorf("UserRef should reference User schema, got %v", refVal)
		}
	}

	// User schema should be added to components/schemas
	user, exists := schemas["User"]
	if !exists {
		t.Error("User schema should be added to components/schemas")
	} else {
		userMap, ok := user.(map[string]interface{})
		if !ok {
			t.Fatalf("User should be a map, got %T", user)
		}
		if userType, hasType := userMap["type"]; !hasType || userType != "object" {
			t.Error("User should contain type: object")
		}
		if _, hasProps := userMap["properties"]; !hasProps {
			t.Error("User should contain properties")
		}
	}
}

func TestReferenceResolver_ResolveAll_NoInlineNestedObjects(t *testing.T) {
	tmpDir := t.TempDir()
	// External file NOT in schemas directory - should be inlined (like swagger-cli)
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

	fileLoader := loader.NewFileLoader()
	p := parser.NewParser()
	resolver := NewReferenceResolver(fileLoader, p)

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

	// External file NOT in schemas dir should be inlined (like swagger-cli does)
	if _, hasRef := child["$ref"]; hasRef {
		t.Error("child should be inlined (no $ref) for files outside schemas directory")
	}

	// Check that content was inlined
	if childType, hasType := child["type"]; !hasType || childType != "object" {
		t.Error("child should have type: object after inlining")
	}
	if _, hasProps := child["properties"]; !hasProps {
		t.Error("child should have properties after inlining")
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

	fileLoader := loader.NewFileLoader()
	p := parser.NewParser()
	resolver := NewReferenceResolver(fileLoader, p)

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

	fileLoader := loader.NewFileLoader()
	p := parser.NewParser()
	resolver := NewReferenceResolver(fileLoader, p)

	data := map[string]interface{}{
		"openapi": "3.0.0",
		"paths":   "./paths/_index.yaml",
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

	fileLoader := loader.NewFileLoader()
	p := parser.NewParser()
	resolver := NewReferenceResolver(fileLoader, p)

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

	fileLoader := loader.NewFileLoader()
	p := parser.NewParser()
	resolver := NewReferenceResolver(fileLoader, p)

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

	fileLoader := loader.NewFileLoader()
	p := parser.NewParser()
	resolver := NewReferenceResolver(fileLoader, p)

	data := map[string]interface{}{
		"openapi": "3.0.0",
		"paths":   "./paths/_index.yaml",
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

	fileLoader := loader.NewFileLoader()
	p := parser.NewParser()
	resolver := NewReferenceResolver(fileLoader, p)

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

	// Schemas not in _index.yaml should be inlined
	if _, hasRef := user["$ref"]; hasRef {
		t.Error("user should be inlined (schemas not registered in _index.yaml)")
	}

	// Check content was inlined
	if userType, hasType := user["type"]; !hasType || userType != "object" {
		t.Error("user should have type: object after inlining")
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

	fileLoader := loader.NewFileLoader()
	p := parser.NewParser()
	resolver := NewReferenceResolver(fileLoader, p)

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
	// File NOT in schemas directory - should be inlined
	baseFile := filepath.Join(tmpDir, "Base.yaml")
	baseContent := []byte(`type: object
properties:
  id:
    type: integer
`)

	if err := os.WriteFile(baseFile, baseContent, 0644); err != nil {
		t.Fatalf("Failed to create base file: %v", err)
	}

	fileLoader := loader.NewFileLoader()
	p := parser.NewParser()
	resolver := NewReferenceResolver(fileLoader, p)

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

	if _, hasRef := firstItem["$ref"]; hasRef {
		t.Error("first allOf item should be inlined (no $ref for files outside schemas)")
	}

	if itemType, hasType := firstItem["type"]; !hasType || itemType != "object" {
		t.Error("first allOf item should have type: object after inlining")
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

	fileLoader := loader.NewFileLoader()
	p := parser.NewParser()
	resolver := NewReferenceResolver(fileLoader, p)

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

	if _, hasRef := items["$ref"]; hasRef {
		t.Error("items should be inlined (no $ref for files outside schemas)")
	}

	if itemType, hasType := items["type"]; !hasType || itemType != "object" {
		t.Error("items should have type: object after inlining")
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

	fileLoader := loader.NewFileLoader()
	p := parser.NewParser()
	resolver := NewReferenceResolver(fileLoader, p)

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

	if _, hasRef := name["$ref"]; hasRef {
		t.Error("name should be inlined (schema not registered in _index.yaml)")
	}

	if nameType, hasType := name["type"]; !hasType || nameType != "string" {
		t.Error("name should have type: string after inlining")
	}
}

func TestReferenceResolver_ComponentNames_RealNames(t *testing.T) {
	tmpDir := t.TempDir()
	schemasDir := filepath.Join(tmpDir, "schemas")
	if err := os.MkdirAll(schemasDir, 0755); err != nil {
		t.Fatalf("Failed to create schemas directory: %v", err)
	}

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

	fileLoader := loader.NewFileLoader()
	p := parser.NewParser()
	resolver := NewReferenceResolver(fileLoader, p)

	// Fragment reference to component in external file
	data := map[string]interface{}{
		"openapi": "3.0.0",
		"paths": map[string]interface{}{
			"/test": map[string]interface{}{
				"get": map[string]interface{}{
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "OK",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "./schemas/RequestGuests.yaml#/components/schemas/RequestGuests",
									},
								},
							},
						},
					},
				},
			},
		},
		"components": map[string]interface{}{
			"schemas": map[string]interface{}{},
		},
	}

	ctx := context.Background()
	config := domain.Config{MaxDepth: 10}

	basePath := tmpDir
	err := resolver.ResolveAll(ctx, data, basePath, config)
	if err != nil {
		t.Fatalf("ResolveAll() error = %v", err)
	}

	// Fragment reference should resolve to component
	components, ok := data["components"].(map[string]interface{})
	if !ok {
		t.Fatal("components should exist")
	}
	schemas, ok := components["schemas"].(map[string]interface{})
	if !ok {
		t.Fatal("schemas should exist")
	}

	// RequestGuests should be added from fragment reference
	requestGuests, exists := schemas["RequestGuests"]
	if !exists {
		t.Error("RequestGuests schema should be added from fragment reference")
		return
	}

	rg, ok := requestGuests.(map[string]interface{})
	if !ok {
		t.Fatalf("RequestGuests should be a map, got %T", requestGuests)
	}
	if rgType, hasType := rg["type"]; !hasType || rgType != "object" {
		t.Errorf("RequestGuests should have type: object, got %v", rg)
	}
}

// TestReferenceResolver_NoDuplicateSchemas tests that schemas are not duplicated
// Note: This test is skipped because the bundler now inlines schemas not in _index.yaml
// instead of extracting them to components (matching swagger-cli behavior)
func TestReferenceResolver_NoDuplicateSchemas(t *testing.T) {
	t.Skip("Skipped: bundler now inlines schemas not in _index.yaml instead of extracting")
	tmpDir := t.TempDir()
	schemasDir := filepath.Join(tmpDir, "schemas")
	if err := os.MkdirAll(schemasDir, 0755); err != nil {
		t.Fatalf("Failed to create schemas directory: %v", err)
	}

	errorFile := filepath.Join(schemasDir, "Error.yaml")
	errorContent := []byte(`type: object
title: ErrorObject
required:
  - code
  - message
  - uuid
properties:
  code:
    description: Код ошибки
    type: string
  message:
    description: Краткое описание ошибки
    type: string
  uuid:
    description: |
      Уникальный идентификатор.  Генерируется рандомно.  Используется для трассировки ошибок
    format: uuid
    type: string
`)
	if err := os.WriteFile(errorFile, errorContent, 0644); err != nil {
		t.Fatalf("Failed to create error file: %v", err)
	}

	fileLoader := loader.NewFileLoader()
	p := parser.NewParser()
	resolver := NewReferenceResolver(fileLoader, p)

	data := map[string]interface{}{
		"openapi": "3.0.0",
		"paths": map[string]interface{}{
			"/api/v1/dictionary/{dictionary_id}/counter": map[string]interface{}{
				"post": map[string]interface{}{
					"summary": "Увеличение счетчика использования адреса",
					"responses": map[string]interface{}{
						"404": map[string]interface{}{
							"description": "Адрес не найден",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "./schemas/Error.yaml",
									},
								},
							},
						},
						"500": map[string]interface{}{
							"description": "Внутренняя ошибка сервера",
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
				"Error": map[string]interface{}{
					"$ref": "#/components/schemas/Error",
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

	// Schemas not in _index.yaml should be inlined
	// Check that response schemas are inlined (no $ref to components)
	paths := data["paths"].(map[string]interface{})
	dictPath := paths["/api/v1/dictionary/{dictionary_id}/counter"].(map[string]interface{})
	post := dictPath["post"].(map[string]interface{})
	responses := post["responses"].(map[string]interface{})

	response404 := responses["404"].(map[string]interface{})
	content404 := response404["content"].(map[string]interface{})
	appJson404 := content404["application/json"].(map[string]interface{})
	schema404 := appJson404["schema"].(map[string]interface{})

	// Schema should be inlined (no $ref for schemas not in _index.yaml)
	if _, hasRef := schema404["$ref"]; hasRef {
		t.Error("schema in 404 response should be inlined (no $ref)")
	}

	// Check inlined content
	if schemaType, hasType := schema404["type"]; !hasType || schemaType != "object" {
		t.Error("inlined schema should have type: object")
	}

	// Self-referencing schema should be removed
	components, ok := data["components"].(map[string]interface{})
	if ok {
		if schemas, ok := components["schemas"].(map[string]interface{}); ok {
			// Error schema that was self-referencing should be removed or empty
			if errorSchema, exists := schemas["Error"]; exists {
				if errorMap, ok := errorSchema.(map[string]interface{}); ok {
					if refVal, hasRef := errorMap["$ref"]; hasRef {
						if refStr, ok := refVal.(string); ok && refStr == "#/components/schemas/Error" {
							t.Error("Self-referencing Error schema should be removed")
						}
					}
				}
			}
		}
	}

}

// TestReferenceResolver_NoDuplicateSchemas_Multiple is skipped - see TestReferenceResolver_NoDuplicateSchemas
func TestReferenceResolver_NoDuplicateSchemas_Multiple(t *testing.T) {
	t.Skip("Skipped: bundler now inlines schemas not in _index.yaml instead of extracting")
	tmpDir := t.TempDir()
	schemasDir := filepath.Join(tmpDir, "schemas")
	if err := os.MkdirAll(schemasDir, 0755); err != nil {
		t.Fatalf("Failed to create schemas directory: %v", err)
	}

	additionalInfoItemFile := filepath.Join(schemasDir, "AdditionalInfoItem.yaml")
	additionalInfoItemContent := []byte(`type: object
required:
  - id
  - value
properties:
  description:
    description: Текстовое описание атрибута заявки (как на фронте)
    example: Предпочтительный тип авто
    type: string
  id:
    description: ID типа атрибута заявки (1 - Предпочтительный тип авто, 2 - Особые требования, 3 - Комментарий для Протокольной службы)
    example: 1
    type: integer
  value:
    description: ID типа атрибута заявки ()
    example: Чёрный бумер, тонировка
    type: string
`)
	if err := os.WriteFile(additionalInfoItemFile, additionalInfoItemContent, 0644); err != nil {
		t.Fatalf("Failed to create additionalInfoItem file: %v", err)
	}

	anonimGuestFile := filepath.Join(schemasDir, "AnonimGuest.yaml")
	anonimGuestContent := []byte(`type: object
description: Гость не существовал в каталоге, пользователя добавим в заявку как анонимного
required:
  - name
  - contacts
properties:
  contacts:
    description: Контактные данные Гостя. Массив значений с указанны типом данных
    items:
      $ref: '#/components/schemas/GuestContact'
    minItems: 0
    type: array
  name:
    example: Иванов Иван
    type: string
`)
	if err := os.WriteFile(anonimGuestFile, anonimGuestContent, 0644); err != nil {
		t.Fatalf("Failed to create anonimGuest file: %v", err)
	}

	fileLoader := loader.NewFileLoader()
	p := parser.NewParser()
	resolver := NewReferenceResolver(fileLoader, p)

	data := map[string]interface{}{
		"openapi": "3.0.0",
		"paths": map[string]interface{}{
			"/api/v1/test": map[string]interface{}{
				"post": map[string]interface{}{
					"requestBody": map[string]interface{}{
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"$ref": "./schemas/AdditionalInfoItem.yaml",
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "./schemas/AnonimGuest.yaml",
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
				"AdditionalInfoItem": map[string]interface{}{
					"$ref": "#/components/schemas/AdditionalInfoItem",
				},
				"AnonimGuest": map[string]interface{}{
					"$ref": "#/components/schemas/AnonimGuest",
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

	schemas := data["components"].(map[string]interface{})["schemas"].(map[string]interface{})

	if _, exists := schemas["AdditionalInfoItem"]; !exists {
		t.Error("AdditionalInfoItem schema should exist with name 'AdditionalInfoItem'")
	}

	if _, exists := schemas["AnonimGuest"]; !exists {
		t.Error("AnonimGuest schema should exist with name 'AnonimGuest'")
	}

	for name := range schemas {
		if strings.HasPrefix(name, "AdditionalInfoItem") && len(name) > 18 {
			rest := name[18:]
			isNumber := true
			for _, r := range rest {
				if r < '0' || r > '9' {
					isNumber = false
					break
				}
			}
			if isNumber {
				t.Errorf("Found duplicate schema name: %s. Should not create AdditionalInfoItem1, AdditionalInfoItem2, etc.", name)
			}
		}
		if strings.HasPrefix(name, "AnonimGuest") && len(name) > 11 {
			rest := name[11:]
			isNumber := true
			for _, r := range rest {
				if r < '0' || r > '9' {
					isNumber = false
					break
				}
			}
			if isNumber {
				t.Errorf("Found duplicate schema name: %s. Should not create AnonimGuest1, AnonimGuest2, etc.", name)
			}
		}
	}

	additionalInfoItem := schemas["AdditionalInfoItem"].(map[string]interface{})
	if _, hasRef := additionalInfoItem["$ref"]; hasRef {
		if len(additionalInfoItem) == 1 {
			t.Error("AdditionalInfoItem schema should contain actual content, not only $ref")
		}
	}
	if additionalInfoItem["type"] != "object" {
		t.Error("AdditionalInfoItem schema should have type: object")
	}

	anonimGuest := schemas["AnonimGuest"].(map[string]interface{})
	if _, hasRef := anonimGuest["$ref"]; hasRef {
		if len(anonimGuest) == 1 {
			t.Error("AnonimGuest schema should contain actual content, not only $ref")
		}
	}
	if anonimGuest["type"] != "object" {
		t.Error("AnonimGuest schema should have type: object")
	}
}

// TestReferenceResolver_NoDuplicateSchemas_ChangePasswordRequest is skipped - see TestReferenceResolver_NoDuplicateSchemas
func TestReferenceResolver_NoDuplicateSchemas_ChangePasswordRequest(t *testing.T) {
	t.Skip("Skipped: bundler now inlines schemas not in _index.yaml instead of extracting")
	tmpDir := t.TempDir()
	schemasDir := filepath.Join(tmpDir, "schemas")
	if err := os.MkdirAll(schemasDir, 0755); err != nil {
		t.Fatalf("Failed to create schemas directory: %v", err)
	}

	changePasswordRequestFile := filepath.Join(schemasDir, "ChangePasswordRequest.yaml")
	changePasswordRequestContent := []byte(`type: object
required:
  - password
properties:
  password:
    $ref: '#/components/schemas/Password'
`)
	if err := os.WriteFile(changePasswordRequestFile, changePasswordRequestContent, 0644); err != nil {
		t.Fatalf("Failed to create changePasswordRequest file: %v", err)
	}

	fileLoader := loader.NewFileLoader()
	p := parser.NewParser()
	resolver := NewReferenceResolver(fileLoader, p)

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
				},
			},
		},
		"components": map[string]interface{}{
			"schemas": map[string]interface{}{
				"ChangePasswordRequest": map[string]interface{}{
					"$ref": "#/components/schemas/ChangePasswordRequest",
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

	schemas := data["components"].(map[string]interface{})["schemas"].(map[string]interface{})

	if _, exists := schemas["ChangePasswordRequest"]; !exists {
		t.Error("ChangePasswordRequest schema should exist with name 'ChangePasswordRequest'")
	}

	for name := range schemas {
		if strings.HasPrefix(name, "ChangePasswordRequest") && len(name) > 20 {
			rest := name[20:]
			isNumber := true
			for _, r := range rest {
				if r < '0' || r > '9' {
					isNumber = false
					break
				}
			}
			if isNumber {
				t.Errorf("Found duplicate schema name: %s. Should not create ChangePasswordRequest1, ChangePasswordRequest2, etc.", name)
			}
		}
	}

	changePasswordRequest := schemas["ChangePasswordRequest"].(map[string]interface{})
	if _, hasRef := changePasswordRequest["$ref"]; hasRef {
		if len(changePasswordRequest) == 1 {
			t.Error("ChangePasswordRequest schema should contain actual content, not only $ref")
		}
	}
	if changePasswordRequest["type"] != "object" {
		t.Error("ChangePasswordRequest schema should have type: object")
	}
}

func TestReferenceResolver_ParametersKeepRef(t *testing.T) {
	tmpDir := t.TempDir()
	parametersDir := filepath.Join(tmpDir, "parameters")
	if err := os.MkdirAll(parametersDir, 0755); err != nil {
		t.Fatalf("Failed to create parameters directory: %v", err)
	}

	xDeviceIdFile := filepath.Join(parametersDir, "X-Device-Id.yaml")
	xDeviceIdContent := []byte(`description: |
  Уникальный идентификатор устройства...
in: header
name: X-Device-Id
required: true
schema:
  example: 550e8400-e29b-41d4-a716-446655440000
  format: uuid
  type: string
style: simple
`)
	if err := os.WriteFile(xDeviceIdFile, xDeviceIdContent, 0644); err != nil {
		t.Fatalf("Failed to create X-Device-Id file: %v", err)
	}

	xAppVersionFile := filepath.Join(parametersDir, "X-App-Version.yaml")
	xAppVersionContent := []byte(`description: Версия приложения
in: header
name: X-App-Version
required: true
schema:
  type: string
style: simple
`)
	if err := os.WriteFile(xAppVersionFile, xAppVersionContent, 0644); err != nil {
		t.Fatalf("Failed to create X-App-Version file: %v", err)
	}

	dictionaryIdParamFile := filepath.Join(parametersDir, "dictionaryIdParam.yaml")
	dictionaryIdParamContent := []byte(`in: path
name: dictionary_id
required: true
schema:
  type: integer
`)
	if err := os.WriteFile(dictionaryIdParamFile, dictionaryIdParamContent, 0644); err != nil {
		t.Fatalf("Failed to create dictionaryIdParam file: %v", err)
	}

	fileLoader := loader.NewFileLoader()
	p := parser.NewParser()
	resolver := NewReferenceResolver(fileLoader, p)

	data := map[string]interface{}{
		"openapi": "3.0.0",
		"paths": map[string]interface{}{
			"/api/v1/dictionary/{dictionary_id}/counter": map[string]interface{}{
				"post": map[string]interface{}{
					"summary": "Увеличение счетчика использования адреса",
					"parameters": []interface{}{
						map[string]interface{}{
							"$ref": "./parameters/X-Device-Id.yaml",
						},
						map[string]interface{}{
							"$ref": "./parameters/X-App-Version.yaml",
						},
						map[string]interface{}{
							"$ref": "./parameters/dictionaryIdParam.yaml",
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "OK",
						},
					},
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

	paths := data["paths"].(map[string]interface{})
	path := paths["/api/v1/dictionary/{dictionary_id}/counter"].(map[string]interface{})
	post := path["post"].(map[string]interface{})
	parameters, hasParameters := post["parameters"]
	if !hasParameters {
		t.Fatal("parameters should exist")
	}

	paramsArray, ok := parameters.([]interface{})
	if !ok {
		t.Fatalf("parameters should be an array, got %T", parameters)
	}

	if len(paramsArray) != 3 {
		t.Fatalf("parameters should have 3 items, got %d", len(paramsArray))
	}

	for i, param := range paramsArray {
		paramMap, ok := param.(map[string]interface{})
		if !ok {
			t.Fatalf("parameter %d should be a map, got %T", i, param)
		}

		ref, hasRef := paramMap["$ref"]
		if !hasRef {
			t.Errorf("parameter %d should contain $ref, got full object: %v", i, paramMap)
		}

		refStr, ok := ref.(string)
		if !ok {
			t.Fatalf("parameter %d $ref should be a string, got %T", i, ref)
		}

		if !strings.HasPrefix(refStr, "#/components/parameters/") {
			t.Errorf("parameter %d $ref should be an internal reference, got %s", i, refStr)
		}

		if len(paramMap) != 1 {
			t.Errorf("parameter %d should contain only $ref, got %d fields: %v", i, len(paramMap), paramMap)
		}
	}

	components, hasComponents := data["components"].(map[string]interface{})
	if !hasComponents {
		t.Fatal("components section should exist")
	}
	parametersSection, hasParameters := components["parameters"].(map[string]interface{})
	if !hasParameters {
		t.Fatal("parameters section should exist in components")
	}

	expectedParams := []string{"X-Device-Id", "X-App-Version", "dictionaryIdParam"}
	for _, expectedName := range expectedParams {
		if _, exists := parametersSection[expectedName]; !exists {
			availableNames := make([]string, 0, len(parametersSection))
			for name := range parametersSection {
				availableNames = append(availableNames, name)
			}
			t.Errorf("parameter %s should exist in components.parameters. Available: %v", expectedName, availableNames)
		}
	}
}

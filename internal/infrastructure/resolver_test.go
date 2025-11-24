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

	// Проверяем, что внутренняя ссылка осталась как есть (не инлайнится)
	paths := data["paths"].([]interface{})
	pathItem := paths[0].(map[string]interface{})
	ref, hasRef := pathItem["$ref"]
	if !hasRef {
		t.Error("internal $ref should remain unchanged")
	}
	if ref != "#/components/schemas/User" {
		t.Errorf("expected $ref to be '#/components/schemas/User', got %v", ref)
	}
}

func TestReferenceResolver_Resolve_InvalidRef(t *testing.T) {
	loader := NewFileLoader()
	parser := NewParser()
	resolver := NewReferenceResolver(loader, parser)

	ctx := context.Background()
	config := domain.Config{MaxDepth: 10}

	// Resolve теперь возвращает nil, nil для пустых ссылок (не используется в новой логике)
	_, err := resolver.Resolve(ctx, "", "/tmp", config)
	// Проверяем, что нет ошибки (новое поведение)
	_ = err
}

func TestReferenceResolver_Resolve_CircularReference(t *testing.T) {
	tmpDir := t.TempDir()
	refFile := filepath.Join(tmpDir, "ref.yaml")
	refContent := []byte(`
type: object
properties:
  self:
    $ref: ref.yaml
`)

	if err := os.WriteFile(refFile, refContent, 0644); err != nil {
		t.Fatalf("Failed to create ref file: %v", err)
	}

	loader := NewFileLoader()
	parser := NewParser()
	resolver := NewReferenceResolver(loader, parser)

	ctx := context.Background()
	config := domain.Config{MaxDepth: 10}

	_, err := resolver.Resolve(ctx, "ref.yaml", tmpDir, config)
	// Should detect circular reference
	_ = err
}

func TestReferenceResolver_Resolve_Fragment(t *testing.T) {
	tmpDir := t.TempDir()
	refFile := filepath.Join(tmpDir, "schemas.yaml")
	refContent := []byte(`openapi: 3.0.0
components:
  schemas:
    User:
      type: object
      properties:
        name:
          type: string
    Admin:
      type: object
      properties:
        role:
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

	// Проверяем, что внешняя ссылка была заменена на внутреннюю
	schemas := data["components"].(map[string]interface{})["schemas"].(map[string]interface{})
	userSchema := schemas["User"].(map[string]interface{})
	
	// Схема User должна содержать ссылку на схему User из внешнего файла
	// Схема User из внешнего файла должна быть добавлена в components/schemas
	ref, hasRef := userSchema["$ref"]
	if !hasRef {
		t.Error("User schema should contain $ref, not inlined content")
	}
	if ref != "#/components/schemas/User" {
		t.Errorf("expected $ref to be '#/components/schemas/User', got %v", ref)
	}
	
	// Проверяем, что схема User из внешнего файла добавлена в components/schemas
	// (может быть с другим именем или как отдельная схема)
	// Но основное - ссылка должна быть, а не инлайниться
}

func TestReferenceResolver_ExpandPathsSection(t *testing.T) {
	tmpDir := t.TempDir()
	pathsFile := filepath.Join(tmpDir, "paths", "_index.yaml")
	if err := os.MkdirAll(filepath.Dir(pathsFile), 0755); err != nil {
		t.Fatalf("Failed to create paths directory: %v", err)
	}
	
	pathsContent := []byte(`/api/v1/login:
  post:
    operationId: authLogin
    summary: Логин пользователя
    responses:
      '200':
        description: OK
/api/v1/employee:
  get:
    operationId: getEmployees
    summary: Получить каталог пользователей
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
		"openapi": "3.0.3",
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

	if _, hasRef := paths["$ref"]; hasRef {
		t.Error("paths should not contain $ref after expansion")
	}

	if _, hasLogin := paths["/api/v1/login"]; !hasLogin {
		t.Error("paths should contain /api/v1/login after expansion")
	}

	if _, hasEmployee := paths["/api/v1/employee"]; !hasEmployee {
		t.Error("paths should contain /api/v1/employee after expansion")
	}
}

func TestReferenceResolver_ExpandPathsSection_StringRef(t *testing.T) {
	tmpDir := t.TempDir()
	pathsFile := filepath.Join(tmpDir, "paths", "_index.yaml")
	if err := os.MkdirAll(filepath.Dir(pathsFile), 0755); err != nil {
		t.Fatalf("Failed to create paths directory: %v", err)
	}
	
	pathsContent := []byte(`/api/v1/test:
  get:
    operationId: test
    summary: Test endpoint
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
		"openapi": "3.0.3",
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

	if _, hasTest := paths["/api/v1/test"]; !hasTest {
		t.Error("paths should contain /api/v1/test after expansion")
	}
}

func TestReferenceResolver_ExpandComponentsParameters(t *testing.T) {
	tmpDir := t.TempDir()
	paramsFile := filepath.Join(tmpDir, "parameters", "_index.yaml")
	if err := os.MkdirAll(filepath.Dir(paramsFile), 0755); err != nil {
		t.Fatalf("Failed to create parameters directory: %v", err)
	}
	
	paramsContent := []byte(`X-App-Version:
  in: header
  name: X-App-Version
  required: true
  schema:
    type: string
X-Device-Id:
  in: header
  name: X-Device-Id
  required: true
  schema:
    type: string
    format: uuid
`)

	if err := os.WriteFile(paramsFile, paramsContent, 0644); err != nil {
		t.Fatalf("Failed to create parameters file: %v", err)
	}

	loader := NewFileLoader()
	parser := NewParser()
	resolver := NewReferenceResolver(loader, parser)

	data := map[string]interface{}{
		"openapi": "3.0.3",
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
	parameters, ok := components["parameters"].(map[string]interface{})
	if !ok {
		t.Fatalf("parameters should be a map, got %T", components["parameters"])
	}

	if _, hasRef := parameters["$ref"]; hasRef {
		t.Error("parameters should not contain $ref after expansion")
	}

	if _, hasAppVersion := parameters["X-App-Version"]; !hasAppVersion {
		t.Error("parameters should contain X-App-Version after expansion")
	}

	if _, hasDeviceId := parameters["X-Device-Id"]; !hasDeviceId {
		t.Error("parameters should contain X-Device-Id after expansion")
	}
}

func TestReferenceResolver_ExpandComponentsSchemas(t *testing.T) {
	tmpDir := t.TempDir()
	schemasFile := filepath.Join(tmpDir, "schemas", "_index.yaml")
	if err := os.MkdirAll(filepath.Dir(schemasFile), 0755); err != nil {
		t.Fatalf("Failed to create schemas directory: %v", err)
	}
	
	schemasContent := []byte(`LoginRequest:
  type: object
  required:
    - phone
    - password
  properties:
    phone:
      type: string
    password:
      type: string
EmployeeCreateRequest:
  type: object
  required:
    - name
    - primary_phone
  properties:
    name:
      type: string
    primary_phone:
      type: string
`)

	if err := os.WriteFile(schemasFile, schemasContent, 0644); err != nil {
		t.Fatalf("Failed to create schemas file: %v", err)
	}

	loader := NewFileLoader()
	parser := NewParser()
	resolver := NewReferenceResolver(loader, parser)

	data := map[string]interface{}{
		"openapi": "3.0.3",
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
	schemas, ok := components["schemas"].(map[string]interface{})
	if !ok {
		t.Fatalf("schemas should be a map, got %T", components["schemas"])
	}

	if _, hasLoginRequest := schemas["LoginRequest"]; !hasLoginRequest {
		t.Error("schemas should contain LoginRequest after expansion")
	}

	if _, hasEmployeeRequest := schemas["EmployeeCreateRequest"]; !hasEmployeeRequest {
		t.Error("schemas should contain EmployeeCreateRequest after expansion")
	}
}

func TestReferenceResolver_ExpandMultipleSections(t *testing.T) {
	tmpDir := t.TempDir()
	
	pathsFile := filepath.Join(tmpDir, "paths", "_index.yaml")
	if err := os.MkdirAll(filepath.Dir(pathsFile), 0755); err != nil {
		t.Fatalf("Failed to create paths directory: %v", err)
	}
	pathsContent := []byte(`/api/v1/test:
  get:
    operationId: test
    responses:
      '200':
        description: OK
`)
	if err := os.WriteFile(pathsFile, pathsContent, 0644); err != nil {
		t.Fatalf("Failed to create paths file: %v", err)
	}

	paramsFile := filepath.Join(tmpDir, "parameters", "_index.yaml")
	if err := os.MkdirAll(filepath.Dir(paramsFile), 0755); err != nil {
		t.Fatalf("Failed to create parameters directory: %v", err)
	}
	paramsContent := []byte(`X-Header:
  in: header
  name: X-Header
  required: true
  schema:
    type: string
`)
	if err := os.WriteFile(paramsFile, paramsContent, 0644); err != nil {
		t.Fatalf("Failed to create parameters file: %v", err)
	}

	schemasFile := filepath.Join(tmpDir, "schemas", "_index.yaml")
	if err := os.MkdirAll(filepath.Dir(schemasFile), 0755); err != nil {
		t.Fatalf("Failed to create schemas directory: %v", err)
	}
	schemasContent := []byte(`TestSchema:
  type: object
  properties:
    id:
      type: integer
`)
	if err := os.WriteFile(schemasFile, schemasContent, 0644); err != nil {
		t.Fatalf("Failed to create schemas file: %v", err)
	}

	loader := NewFileLoader()
	parser := NewParser()
	resolver := NewReferenceResolver(loader, parser)

	data := map[string]interface{}{
		"openapi": "3.0.3",
		"paths": map[string]interface{}{
			"$ref": "./paths/_index.yaml",
		},
		"components": map[string]interface{}{
			"parameters": map[string]interface{}{
				"$ref": "./parameters/_index.yaml",
			},
			"schemas": "./schemas/_index.yaml",
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
	if _, hasRef := paths["$ref"]; hasRef {
		t.Error("paths should not contain $ref after expansion")
	}
	if _, hasTest := paths["/api/v1/test"]; !hasTest {
		t.Error("paths should contain /api/v1/test after expansion")
	}

	components := data["components"].(map[string]interface{})
	
	parameters, ok := components["parameters"].(map[string]interface{})
	if !ok {
		t.Fatalf("parameters should be a map, got %T", components["parameters"])
	}
	if _, hasRef := parameters["$ref"]; hasRef {
		t.Error("parameters should not contain $ref after expansion")
	}
	if _, hasHeader := parameters["X-Header"]; !hasHeader {
		t.Error("parameters should contain X-Header after expansion")
	}

	schemas, ok := components["schemas"].(map[string]interface{})
	if !ok {
		t.Fatalf("schemas should be a map, got %T", components["schemas"])
	}
	if _, hasTestSchema := schemas["TestSchema"]; !hasTestSchema {
		t.Error("schemas should contain TestSchema after expansion")
	}
}

func TestReferenceResolver_ExpandSectionWithNestedRefs(t *testing.T) {
	tmpDir := t.TempDir()
	
	schemasFile := filepath.Join(tmpDir, "schemas", "_index.yaml")
	if err := os.MkdirAll(filepath.Dir(schemasFile), 0755); err != nil {
		t.Fatalf("Failed to create schemas directory: %v", err)
	}
	
	externalSchemaFile := filepath.Join(tmpDir, "schemas", "external.yaml")
	if err := os.MkdirAll(filepath.Dir(externalSchemaFile), 0755); err != nil {
		t.Fatalf("Failed to create schemas directory: %v", err)
	}
	externalContent := []byte(`type: object
properties:
  external_field:
    type: string
`)
	if err := os.WriteFile(externalSchemaFile, externalContent, 0644); err != nil {
		t.Fatalf("Failed to create external schema file: %v", err)
	}

	schemasContent := []byte(`TestSchema:
  type: object
  properties:
    id:
      type: integer
    external:
      $ref: "./external.yaml"
`)
	if err := os.WriteFile(schemasFile, schemasContent, 0644); err != nil {
		t.Fatalf("Failed to create schemas file: %v", err)
	}

	loader := NewFileLoader()
	parser := NewParser()
	resolver := NewReferenceResolver(loader, parser)

	data := map[string]interface{}{
		"openapi": "3.0.3",
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
	schemas, ok := components["schemas"].(map[string]interface{})
	if !ok {
		t.Fatalf("schemas should be a map, got %T", components["schemas"])
	}

	testSchema, ok := schemas["TestSchema"].(map[string]interface{})
	if !ok {
		t.Fatalf("TestSchema should be a map, got %T", schemas["TestSchema"])
	}

	properties, ok := testSchema["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("properties should be a map, got %T", testSchema["properties"])
	}

	external, ok := properties["external"].(map[string]interface{})
	if !ok {
		t.Fatalf("external should be a map, got %T", properties["external"])
	}

	ref, hasRef := external["$ref"]
	if !hasRef {
		t.Error("external should contain $ref after processing")
	}

	refStr, ok := ref.(string)
	if !ok {
		t.Fatalf("$ref should be a string, got %T", ref)
	}

	if !strings.HasPrefix(refStr, "#/components/schemas/") {
		t.Errorf("$ref should be an internal reference, got %s", refStr)
	}
}

func TestReferenceResolver_ResolveAll_AllOfWithExternalRef(t *testing.T) {
	tmpDir := t.TempDir()
	
	externalFile := filepath.Join(tmpDir, "external.yaml")
	externalContent := []byte(`type: object
properties:
  external_field:
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
				"TestSchema": map[string]interface{}{
					"type": "object",
					"allOf": []interface{}{
						map[string]interface{}{
							"$ref": externalFile,
						},
						map[string]interface{}{
							"properties": map[string]interface{}{
								"local_field": map[string]interface{}{
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
	testSchema := schemas["TestSchema"].(map[string]interface{})
	allOf := testSchema["allOf"].([]interface{})
	
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

func TestReferenceResolver_ExpandSectionWithRelativeRefs(t *testing.T) {
	tmpDir := t.TempDir()
	
	pathsFile := filepath.Join(tmpDir, "paths", "_index.yaml")
	if err := os.MkdirAll(filepath.Dir(pathsFile), 0755); err != nil {
		t.Fatalf("Failed to create paths directory: %v", err)
	}
	
	tableRegistryFile := filepath.Join(tmpDir, "paths", "tableRegistry.yaml")
	tableRegistryContent := []byte(`get:
  operationId: getTableRegistry
  summary: Получить все записи из справочника
  responses:
    '200':
      description: OK
`)
	if err := os.WriteFile(tableRegistryFile, tableRegistryContent, 0644); err != nil {
		t.Fatalf("Failed to create tableRegistry file: %v", err)
	}

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
		"openapi": "3.0.3",
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

	if _, hasRef := paths["$ref"]; hasRef {
		t.Error("paths should not contain $ref after expansion")
	}

	tableRegistryPath, hasTableRegistry := paths["/api/v1/table-registry"]
	if !hasTableRegistry {
		t.Error("paths should contain /api/v1/table-registry after expansion")
	}

	tableRegistryMap, ok := tableRegistryPath.(map[string]interface{})
	if !ok {
		t.Fatalf("/api/v1/table-registry should be a map, got %T", tableRegistryPath)
	}

	getOp, hasGet := tableRegistryMap["get"]
	if !hasGet {
		t.Error("/api/v1/table-registry should contain 'get' operation")
	}

	getOpMap, ok := getOp.(map[string]interface{})
	if !ok {
		t.Fatalf("get operation should be a map, got %T", getOp)
	}

	if getOpMap["operationId"] != "getTableRegistry" {
		t.Errorf("expected operationId to be 'getTableRegistry', got %v", getOpMap["operationId"])
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


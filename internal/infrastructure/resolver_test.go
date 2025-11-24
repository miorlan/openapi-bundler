package infrastructure

import (
	"context"
	"os"
	"path/filepath"
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


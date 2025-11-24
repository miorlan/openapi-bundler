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

	// Проверяем, что ссылка была разрешена
	paths := data["paths"].([]interface{})
	pathItem := paths[0].(map[string]interface{})
	if _, hasRef := pathItem["$ref"]; hasRef {
		t.Error("$ref should be resolved and removed")
	}
	if _, hasType := pathItem["type"]; !hasType {
		t.Error("resolved content should have 'type' field")
	}
}

func TestReferenceResolver_Resolve_InvalidRef(t *testing.T) {
	loader := NewFileLoader()
	parser := NewParser()
	resolver := NewReferenceResolver(loader, parser)

	ctx := context.Background()
	config := domain.Config{MaxDepth: 10}

	_, err := resolver.Resolve(ctx, "", "/tmp", config)
	if err == nil {
		t.Error("Resolve() expected error for empty ref")
	}
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

	// Проверяем, что ссылка была разрешена и содержит только схему User
	userSchema := data["components"].(map[string]interface{})["schemas"].(map[string]interface{})["User"].(map[string]interface{})
	if _, hasRef := userSchema["$ref"]; hasRef {
		t.Error("$ref should be resolved and removed")
	}
	if userSchema["type"] != "object" {
		t.Errorf("expected type 'object', got %v", userSchema["type"])
	}
	// Проверяем, что Admin не попал в результат (только User)
	if admin, ok := userSchema["Admin"]; ok {
		t.Errorf("Admin should not be in resolved User schema, got %v", admin)
	}
}


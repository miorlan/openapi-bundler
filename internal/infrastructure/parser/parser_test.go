package parser

import (
	"testing"

	"github.com/miorlan/openapi-bundler/internal/domain"
)

func TestParser_Unmarshal_YAML(t *testing.T) {
	parser := NewParser()
	data := []byte(`
openapi: 3.0.0
info:
  title: Test API
  version: 1.0.0
`)

	var result map[string]interface{}
	err := parser.Unmarshal(data, &result, domain.FormatYAML)
	if err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if result["openapi"] != "3.0.0" {
		t.Errorf("Unmarshal() openapi = %v, want 3.0.0", result["openapi"])
	}
}

func TestParser_Unmarshal_JSON(t *testing.T) {
	parser := NewParser()
	data := []byte(`{"openapi":"3.0.0","info":{"title":"Test API"}}`)

	var result map[string]interface{}
	err := parser.Unmarshal(data, &result, domain.FormatJSON)
	if err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if result["openapi"] != "3.0.0" {
		t.Errorf("Unmarshal() openapi = %v, want 3.0.0", result["openapi"])
	}
}

func TestParser_Marshal_YAML(t *testing.T) {
	parser := NewParser()
	data := map[string]interface{}{
		"openapi": "3.0.0",
		"info": map[string]interface{}{
			"title": "Test API",
		},
	}

	result, err := parser.Marshal(data, domain.FormatYAML)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	if len(result) == 0 {
		t.Error("Marshal() returned empty result")
	}
}

func TestParser_Marshal_JSON(t *testing.T) {
	parser := NewParser()
	data := map[string]interface{}{
		"openapi": "3.0.0",
		"info": map[string]interface{}{
			"title": "Test API",
		},
	}

	result, err := parser.Marshal(data, domain.FormatJSON)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	if len(result) == 0 {
		t.Error("Marshal() returned empty result")
	}
}

func TestParser_Unmarshal_ByContent_JSON(t *testing.T) {
	parser := NewParser()
	data := []byte(`{"openapi":"3.0.0","info":{"title":"Test"}}`)

	var result map[string]interface{}
	err := parser.Unmarshal(data, &result, "")
	if err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if result["openapi"] != "3.0.0" {
		t.Errorf("Unmarshal() openapi = %v, want 3.0.0", result["openapi"])
	}
}

func TestParser_Unmarshal_ByContent_YAML(t *testing.T) {
	parser := NewParser()
	data := []byte(`
openapi: 3.0.0
info:
  title: Test
`)

	var result map[string]interface{}
	err := parser.Unmarshal(data, &result, "")
	if err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if result["openapi"] != "3.0.0" {
		t.Errorf("Unmarshal() openapi = %v, want 3.0.0", result["openapi"])
	}
}

func TestParser_Unmarshal_EmptyContent(t *testing.T) {
	parser := NewParser()
	data := []byte("")

	var result map[string]interface{}
	err := parser.Unmarshal(data, &result, "")
	if err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
}


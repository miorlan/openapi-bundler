package infrastructure

import (
	"encoding/json"
	"strings"

	"github.com/miorlan/openapi-bundler/internal/domain"
	"gopkg.in/yaml.v3"
)

type Parser struct{}

func NewParser() domain.Parser {
	return &Parser{}
}

func (p *Parser) Unmarshal(data []byte, v interface{}, format domain.FileFormat) error {
	switch format {
	case domain.FormatJSON:
		return json.Unmarshal(data, v)
	case domain.FormatYAML:
		return yaml.Unmarshal(data, v)
	default:
		return p.unmarshalByContent(data, v)
	}
}

func (p *Parser) Marshal(v interface{}, format domain.FileFormat) ([]byte, error) {
	switch format {
	case domain.FormatJSON:
		return json.MarshalIndent(v, "", "  ")
	case domain.FormatYAML:
		// Переупорядочиваем ключи для сохранения стандартного порядка OpenAPI
		if ordered, ok := p.reorderOpenAPIFields(v); ok {
			return yaml.Marshal(ordered)
		}
		return yaml.Marshal(v)
	default:
		// Переупорядочиваем ключи для сохранения стандартного порядка OpenAPI
		if ordered, ok := p.reorderOpenAPIFields(v); ok {
			return yaml.Marshal(ordered)
		}
		return yaml.Marshal(v)
	}
}

// reorderOpenAPIFields переупорядочивает ключи в map согласно стандартному порядку OpenAPI
func (p *Parser) reorderOpenAPIFields(v interface{}) (interface{}, bool) {
	data, ok := v.(map[string]interface{})
	if !ok {
		return v, false
	}

	// Стандартный порядок полей OpenAPI
	fieldOrder := []string{
		"openapi",
		"info",
		"externalDocs",
		"servers",
		"tags",
		"paths",
		"components",
		"security",
		"webhooks",
		"x-", // x-* поля в конце
	}

	ordered := make(map[string]interface{})
	processed := make(map[string]bool)

	// Сначала добавляем поля в стандартном порядке
	for _, key := range fieldOrder {
		if key == "x-" {
			// Добавляем все x-* поля
			for k, val := range data {
				if strings.HasPrefix(k, "x-") && !processed[k] {
					ordered[k] = p.reorderNestedFields(val)
					processed[k] = true
				}
			}
		} else {
			if val, exists := data[key]; exists {
				ordered[key] = p.reorderNestedFields(val)
				processed[key] = true
			}
		}
	}

	// Затем добавляем остальные поля, которых нет в стандартном порядке
	for k, val := range data {
		if !processed[k] {
			ordered[k] = p.reorderNestedFields(val)
		}
	}

	return ordered, true
}

// reorderNestedFields рекурсивно переупорядочивает вложенные структуры
func (p *Parser) reorderNestedFields(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		// Если это components, переупорядочиваем секции компонентов
		if len(val) > 0 {
			// Проверяем, не является ли это components
			_, hasSchemas := val["schemas"]
			_, hasResponses := val["responses"]
			_, hasParameters := val["parameters"]
			if hasSchemas || hasResponses || hasParameters {
				// Это components - переупорядочиваем секции
				return p.reorderComponentsFields(val)
			}
		}
		// Для других map рекурсивно обрабатываем значения
		result := make(map[string]interface{})
		for k, v := range val {
			result[k] = p.reorderNestedFields(v)
		}
		return result
	case []interface{}:
		// Для массивов рекурсивно обрабатываем элементы
		result := make([]interface{}, len(val))
		for i, item := range val {
			result[i] = p.reorderNestedFields(item)
		}
		return result
	default:
		return v
	}
}

// reorderComponentsFields переупорядочивает секции в components согласно стандартному порядку
func (p *Parser) reorderComponentsFields(data map[string]interface{}) map[string]interface{} {
	// Стандартный порядок секций components
	componentOrder := []string{
		"schemas",
		"responses",
		"parameters",
		"examples",
		"requestBodies",
		"headers",
		"securitySchemes",
		"links",
		"callbacks",
	}

	ordered := make(map[string]interface{})
	processed := make(map[string]bool)

	// Сначала добавляем секции в стандартном порядке
	for _, key := range componentOrder {
		if val, exists := data[key]; exists {
			ordered[key] = p.reorderNestedFields(val)
			processed[key] = true
		}
	}

	// Затем добавляем остальные секции
	for k, val := range data {
		if !processed[k] {
			ordered[k] = p.reorderNestedFields(val)
		}
	}

	return ordered
}

func (p *Parser) unmarshalByContent(data []byte, v interface{}) error {
	trimmed := strings.TrimSpace(string(data))
	if len(trimmed) == 0 {
		return yaml.Unmarshal(data, v)
	}

	if trimmed[0] == '{' || trimmed[0] == '[' {
		if err := json.Unmarshal(data, v); err == nil {
			return nil
		}
	}

	return yaml.Unmarshal(data, v)
}


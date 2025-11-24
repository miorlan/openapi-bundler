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
		return yaml.Marshal(v)
	default:
		return yaml.Marshal(v)
	}
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


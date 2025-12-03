package parser

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/miorlan/openapi-bundler/internal/domain"
	"gopkg.in/yaml.v3"
)

// Parser provides parsing functionality that preserves key order
type Parser struct {
	outputFormat domain.FileFormat
}

// NewParser creates a new Parser
func NewParser() *Parser {
	return &Parser{}
}

// SetOutputFormat sets the output format (YAML or JSON)
func (p *Parser) SetOutputFormat(format domain.FileFormat) {
	p.outputFormat = format
}

// ParseFile parses YAML/JSON data into a yaml.Node preserving order
func (p *Parser) ParseFile(data []byte) (*yaml.Node, error) {
	var node yaml.Node
	if err := yaml.Unmarshal(data, &node); err != nil {
		return nil, err
	}
	return &node, nil
}

// MarshalNode marshals a yaml.Node to YAML or JSON bytes
func (p *Parser) MarshalNode(node *yaml.Node) ([]byte, error) {
	if p.outputFormat == domain.FormatJSON {
		return p.marshalJSON(node)
	}
	return p.marshalYAML(node)
}

// marshalYAML marshals to YAML format
func (p *Parser) marshalYAML(node *yaml.Node) ([]byte, error) {
	p.formatNode(node)

	var buf strings.Builder
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(node); err != nil {
		return nil, err
	}
	encoder.Close()

	return []byte(buf.String()), nil
}

// marshalJSON marshals to JSON format preserving key order
func (p *Parser) marshalJSON(node *yaml.Node) ([]byte, error) {
	var buf strings.Builder
	if err := p.writeJSONNode(&buf, node, 0); err != nil {
		return nil, err
	}
	return []byte(buf.String()), nil
}

// writeJSONNode writes a yaml.Node as JSON
func (p *Parser) writeJSONNode(buf *strings.Builder, node *yaml.Node, indent int) error {
	if node == nil {
		buf.WriteString("null")
		return nil
	}

	indentStr := strings.Repeat("  ", indent)
	nextIndent := strings.Repeat("  ", indent+1)

	switch node.Kind {
	case yaml.DocumentNode:
		if len(node.Content) > 0 {
			return p.writeJSONNode(buf, node.Content[0], indent)
		}
		buf.WriteString("null")

	case yaml.MappingNode:
		buf.WriteString("{\n")
		for i := 0; i < len(node.Content); i += 2 {
			if i+1 >= len(node.Content) {
				break
			}
			if i > 0 {
				buf.WriteString(",\n")
			}
			buf.WriteString(nextIndent)
			buf.WriteString(`"`)
			buf.WriteString(escapeJSON(node.Content[i].Value))
			buf.WriteString(`": `)
			if err := p.writeJSONNode(buf, node.Content[i+1], indent+1); err != nil {
				return err
			}
		}
		buf.WriteString("\n")
		buf.WriteString(indentStr)
		buf.WriteString("}")

	case yaml.SequenceNode:
		if len(node.Content) == 0 {
			buf.WriteString("[]")
		} else {
			buf.WriteString("[\n")
			for i, item := range node.Content {
				if i > 0 {
					buf.WriteString(",\n")
				}
				buf.WriteString(nextIndent)
				if err := p.writeJSONNode(buf, item, indent+1); err != nil {
					return err
				}
			}
			buf.WriteString("\n")
			buf.WriteString(indentStr)
			buf.WriteString("]")
		}

	case yaml.ScalarNode:
		return p.writeJSONScalar(buf, node)

	default:
		buf.WriteString(`"`)
		buf.WriteString(escapeJSON(node.Value))
		buf.WriteString(`"`)
	}
	return nil
}

// writeJSONScalar writes a scalar node as JSON
func (p *Parser) writeJSONScalar(buf *strings.Builder, node *yaml.Node) error {
	var v interface{}
	if err := node.Decode(&v); err == nil {
		return p.writeJSONValue(buf, v)
	}
	buf.WriteString(`"`)
	buf.WriteString(escapeJSON(node.Value))
	buf.WriteString(`"`)
	return nil
}

// writeJSONValue writes a Go value as JSON
func (p *Parser) writeJSONValue(buf *strings.Builder, v interface{}) error {
	switch val := v.(type) {
	case nil:
		buf.WriteString("null")
	case bool:
		if val {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}
	case int, int64:
		buf.WriteString(fmt.Sprintf("%d", val))
	case float64:
		if val == float64(int64(val)) {
			buf.WriteString(fmt.Sprintf("%d", int64(val)))
		} else {
			buf.WriteString(fmt.Sprintf("%v", val))
		}
	case string:
		buf.WriteString(`"`)
		buf.WriteString(escapeJSON(val))
		buf.WriteString(`"`)
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return err
		}
		buf.Write(data)
	}
	return nil
}

// escapeJSON escapes a string for JSON
func escapeJSON(s string) string {
	var buf strings.Builder
	for _, r := range s {
		switch r {
		case '"':
			buf.WriteString(`\"`)
		case '\\':
			buf.WriteString(`\\`)
		case '\n':
			buf.WriteString(`\n`)
		case '\r':
			buf.WriteString(`\r`)
		case '\t':
			buf.WriteString(`\t`)
		default:
			if r < 32 {
				buf.WriteString(fmt.Sprintf(`\u%04x`, r))
			} else {
				buf.WriteRune(r)
			}
		}
	}
	return buf.String()
}

// formatNode applies YAML formatting rules
func (p *Parser) formatNode(node *yaml.Node) {
	if node == nil {
		return
	}

	// Remove comments
	node.HeadComment = ""
	node.LineComment = ""
	node.FootComment = ""

	switch node.Kind {
	case yaml.DocumentNode:
		for _, child := range node.Content {
			p.formatNode(child)
		}

	case yaml.MappingNode:
		p.sortHTTPStatusCodes(node)
		p.formatMappingNode(node)

	case yaml.SequenceNode:
		if node.Style == yaml.FlowStyle {
			node.Style = 0 // block style
		}
		for _, child := range node.Content {
			p.formatNode(child)
		}

	case yaml.ScalarNode:
		p.formatScalarNode(node)
	}
}

// formatMappingNode formats a mapping node
func (p *Parser) formatMappingNode(node *yaml.Node) {
	for i := 0; i < len(node.Content); i += 2 {
		if i+1 >= len(node.Content) {
			break
		}
		keyNode := node.Content[i]
		valueNode := node.Content[i+1]

		// Clear key comments
		keyNode.HeadComment = ""
		keyNode.LineComment = ""
		keyNode.FootComment = ""

		// Format key
		p.formatKey(keyNode)

		// Format value based on key
		p.formatValue(keyNode.Value, valueNode)

		p.formatNode(valueNode)
	}
}

// formatKey formats a key node
func (p *Parser) formatKey(node *yaml.Node) {
	key := node.Value

	// Path keys: quote only if they contain {}
	if strings.HasPrefix(key, "/") {
		if strings.Contains(key, "{") {
			node.Style = yaml.SingleQuotedStyle
		} else {
			node.Style = 0
		}
		return
	}

	// HTTP status codes
	if isHTTPStatusCode(key) {
		node.Style = yaml.SingleQuotedStyle
	}
}

// formatValue formats a value node based on its key
func (p *Parser) formatValue(key string, node *yaml.Node) {
	// URL values
	if key == "url" && node.Kind == yaml.ScalarNode {
		if strings.HasPrefix(node.Value, "http://") || strings.HasPrefix(node.Value, "https://") {
			node.Style = yaml.SingleQuotedStyle
		}
	}

	// openapi version - no quotes
	if key == "openapi" && node.Kind == yaml.ScalarNode {
		node.Style = 0
	}

	// required/enum arrays - block style
	if (key == "required" || key == "enum") && node.Kind == yaml.SequenceNode {
		node.Style = 0
	}

	// Scalar formatting
	if node.Kind == yaml.ScalarNode {
		p.formatScalarValue(node)
	}
}

// formatScalarNode formats a standalone scalar node
func (p *Parser) formatScalarNode(node *yaml.Node) {
	// Convert folded (>) to literal (|)
	if node.Style == yaml.FoldedStyle {
		node.Style = yaml.LiteralStyle
	}
	// Remove unnecessary double quotes
	if node.Style == yaml.DoubleQuotedStyle && !needsQuoting(node.Value) {
		node.Style = 0
	}
}

// formatScalarValue formats a scalar value in a mapping
func (p *Parser) formatScalarValue(node *yaml.Node) {
	// Convert folded to literal
	if node.Style == yaml.FoldedStyle {
		node.Style = yaml.LiteralStyle
		return
	}

	// Determine appropriate style
	if shouldUseSingleQuotes(node.Value) {
		node.Style = yaml.SingleQuotedStyle
		return
	}

	if node.Style == yaml.DoubleQuotedStyle {
		if !needsQuoting(node.Value) {
			node.Style = 0
		} else if needsSingleQuotes(node.Value) {
			node.Style = yaml.SingleQuotedStyle
		}
	} else if node.Style == 0 && needsSingleQuotes(node.Value) {
		node.Style = yaml.SingleQuotedStyle
	}
}

// sortHTTPStatusCodes sorts HTTP status codes in a mapping
func (p *Parser) sortHTTPStatusCodes(node *yaml.Node) {
	if node == nil || node.Kind != yaml.MappingNode || len(node.Content) < 4 {
		return
	}

	// Check if all keys are HTTP status codes
	for i := 0; i < len(node.Content); i += 2 {
		if !isHTTPStatusCode(node.Content[i].Value) {
			return
		}
	}

	// Collect pairs
	type pair struct {
		key   *yaml.Node
		value *yaml.Node
	}
	pairs := make([]pair, 0, len(node.Content)/2)
	for i := 0; i < len(node.Content); i += 2 {
		if i+1 < len(node.Content) {
			pairs = append(pairs, pair{node.Content[i], node.Content[i+1]})
		}
	}

	// Sort by status code
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].key.Value < pairs[j].key.Value
	})

	// Rebuild content
	node.Content = make([]*yaml.Node, 0, len(pairs)*2)
	for _, p := range pairs {
		node.Content = append(node.Content, p.key, p.value)
	}
}

// Helper functions

func isHTTPStatusCode(s string) bool {
	if len(s) != 3 {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return s[0] >= '1' && s[0] <= '5'
}

func needsQuoting(s string) bool {
	if s == "" {
		return true
	}

	lower := strings.ToLower(s)
	if lower == "true" || lower == "false" || lower == "null" || lower == "yes" || lower == "no" {
		return true
	}

	specialChars := "#[]{},'&*!|>\"%%@`"
	for _, c := range s {
		if strings.ContainsRune(specialChars, c) {
			return true
		}
	}

	if len(s) > 0 && (s[0] == '-' || s[0] == '?' || s[0] == ' ') {
		return true
	}

	return false
}

func shouldUseSingleQuotes(s string) bool {
	// Dates: YYYY-MM-DD or ISO datetime YYYY-MM-DDTHH:MM:SSZ
	if len(s) >= 10 && s[4] == '-' && s[7] == '-' {
		return true
	}
	// Phone numbers: +...
	if strings.HasPrefix(s, "+") {
		return true
	}
	return false
}

func needsSingleQuotes(s string) bool {
	return strings.Contains(s, ":") || strings.Contains(s, ",")
}

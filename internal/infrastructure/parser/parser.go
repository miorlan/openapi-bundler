package parser

import (
	"encoding/json"
	"strings"

	"github.com/miorlan/openapi-bundler/internal/domain"
	"gopkg.in/yaml.v3"
)

type Parser struct {
	originalRootNode *yaml.Node
}

func NewParser() domain.Parser {
	return &Parser{}
}

func (p *Parser) Unmarshal(data []byte, v interface{}, format domain.FileFormat) error {
	switch format {
	case domain.FormatJSON:
		return json.Unmarshal(data, v)
	case domain.FormatYAML:
		var node yaml.Node
		if err := yaml.Unmarshal(data, &node); err != nil {
			return err
		}
		if p.originalRootNode == nil {
			if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
				p.originalRootNode = node.Content[0]
			} else {
				p.originalRootNode = &node
			}
		}
		return node.Decode(v)
	default:
		return p.unmarshalByContent(data, v)
	}
}

func (p *Parser) Marshal(v interface{}, format domain.FileFormat) ([]byte, error) {
	switch format {
	case domain.FormatJSON:
		return json.MarshalIndent(v, "", "  ")
	case domain.FormatYAML:
		fallthrough
	default:
		result, err := p.marshalYAMLWithOrder(v)
		p.originalRootNode = nil
		return result, err
	}
}

func (p *Parser) marshalYAMLWithOrder(v interface{}) ([]byte, error) {
	var node yaml.Node
	if err := node.Encode(v); err != nil {
		return nil, err
	}

	if p.originalRootNode != nil {
		p.preserveOriginalOrder(&node, p.originalRootNode)
	} else {
		p.reorderYAMLNode(&node)
	}

	var buf strings.Builder
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(&node); err != nil {
		return nil, err
	}
	encoder.Close()

	return []byte(buf.String()), nil
}

func (p *Parser) createKeyNode(key string) *yaml.Node {
	return &yaml.Node{
		Kind:  yaml.ScalarNode,
		Value: key,
	}
}

func (p *Parser) buildNodeMap(node *yaml.Node) map[string]*yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return make(map[string]*yaml.Node)
	}
	expectedSize := len(node.Content) / 2
	if expectedSize < 1 {
		expectedSize = 1
	}
	nodeMap := make(map[string]*yaml.Node, expectedSize)
	for i := 0; i < len(node.Content); i += 2 {
		if i+1 >= len(node.Content) {
			break
		}
		keyNode := node.Content[i]
		valueNode := node.Content[i+1]
		nodeMap[keyNode.Value] = valueNode
	}
	return nodeMap
}

func (p *Parser) preserveOriginalOrder(node *yaml.Node, originalNode *yaml.Node) {
	if node == nil || originalNode == nil || node.Kind != yaml.MappingNode || originalNode.Kind != yaml.MappingNode {
		p.reorderYAMLNode(node)
		return
	}

	nodeMap := p.buildNodeMap(node)
	newContent := make([]*yaml.Node, 0, len(node.Content))
	processed := make(map[string]bool, len(nodeMap))

	for i := 0; i < len(originalNode.Content); i += 2 {
		if i+1 >= len(originalNode.Content) {
			break
		}
		originalKeyNode := originalNode.Content[i]
		originalValueNode := originalNode.Content[i+1]
		key := originalKeyNode.Value

		if valueNode, exists := nodeMap[key]; exists {
			newContent = append(newContent, originalKeyNode, valueNode)
			processed[key] = true

			if key == "components" && originalValueNode.Kind == yaml.MappingNode {
				p.preserveComponentsOrder(valueNode, originalValueNode)
			} else {
				p.preserveOriginalOrder(valueNode, originalValueNode)
			}
		}
	}

	for key, valueNode := range nodeMap {
		if !processed[key] {
			newContent = append(newContent, p.createKeyNode(key), valueNode)
			p.reorderYAMLNode(valueNode)
		}
	}

	node.Content = newContent
}

func (p *Parser) preserveComponentsOrder(node *yaml.Node, originalNode *yaml.Node) {
	if node == nil || originalNode == nil || node.Kind != yaml.MappingNode || originalNode.Kind != yaml.MappingNode {
		p.reorderComponentsYAMLNode(node)
		return
	}

	nodeMap := p.buildNodeMap(node)
	newContent := make([]*yaml.Node, 0, len(node.Content))
	processed := make(map[string]bool, len(nodeMap))

	for i := 0; i < len(originalNode.Content); i += 2 {
		if i+1 >= len(originalNode.Content) {
			break
		}
		originalKeyNode := originalNode.Content[i]
		originalValueNode := originalNode.Content[i+1]
		key := originalKeyNode.Value

		if valueNode, exists := nodeMap[key]; exists {
			newContent = append(newContent, originalKeyNode, valueNode)
			processed[key] = true
			p.preserveOriginalOrder(valueNode, originalValueNode)
		}
	}

	for key, valueNode := range nodeMap {
		if !processed[key] {
			newContent = append(newContent, p.createKeyNode(key), valueNode)
			p.reorderYAMLNode(valueNode)
		}
	}

	node.Content = newContent
}

func (p *Parser) reorderYAMLNode(node *yaml.Node) {
	if node == nil || node.Kind != yaml.MappingNode {
		return
	}

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
	}

	expectedSize := len(node.Content) / 2
	if expectedSize < 1 {
		expectedSize = 1
	}
	nodeMap := make(map[string]*yaml.Node, expectedSize)
	xFields := make([]*yaml.Node, 0, expectedSize/4)

	for i := 0; i < len(node.Content); i += 2 {
		if i+1 >= len(node.Content) {
			break
		}
		keyNode := node.Content[i]
		valueNode := node.Content[i+1]
		key := keyNode.Value
		if strings.HasPrefix(key, "x-") {
			xFields = append(xFields, keyNode, valueNode)
		} else {
			nodeMap[key] = valueNode
		}
	}

	newContent := make([]*yaml.Node, 0, len(nodeMap))
	processed := make(map[string]bool, len(nodeMap))

	for _, key := range fieldOrder {
		if valueNode, exists := nodeMap[key]; exists {
			newContent = append(newContent, p.createKeyNode(key), valueNode)
			processed[key] = true

			if key == "components" {
				p.reorderComponentsYAMLNode(valueNode)
			} else {
				p.reorderYAMLNode(valueNode)
			}
		}
	}

	newContent = append(newContent, xFields...)

	for key, valueNode := range nodeMap {
		if !processed[key] {
			newContent = append(newContent, p.createKeyNode(key), valueNode)
			p.reorderYAMLNode(valueNode)
		}
	}

	node.Content = newContent
}

func (p *Parser) reorderComponentsYAMLNode(node *yaml.Node) {
	if node == nil || node.Kind != yaml.MappingNode {
		return
	}

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

	nodeMap := p.buildNodeMap(node)

	newContent := make([]*yaml.Node, 0, len(nodeMap))
	processed := make(map[string]bool, len(nodeMap))

	for _, key := range componentOrder {
		if valueNode, exists := nodeMap[key]; exists {
			newContent = append(newContent, p.createKeyNode(key), valueNode)
			processed[key] = true
			p.reorderYAMLNode(valueNode)
		}
	}

	for key, valueNode := range nodeMap {
		if !processed[key] {
			newContent = append(newContent, p.createKeyNode(key), valueNode)
			p.reorderYAMLNode(valueNode)
		}
	}

	node.Content = newContent
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


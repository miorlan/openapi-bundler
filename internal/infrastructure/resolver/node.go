package resolver

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// NodeHelper provides utilities for working with yaml.Node while preserving order
type NodeHelper struct{}

// GetMapValue gets a value from a mapping node by key
func (h *NodeHelper) GetMapValue(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i < len(node.Content); i += 2 {
		if i+1 >= len(node.Content) {
			break
		}
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

// SetMapValue sets a value in a mapping node, preserving order
func (h *NodeHelper) SetMapValue(node *yaml.Node, key string, value *yaml.Node) {
	if node == nil || node.Kind != yaml.MappingNode {
		return
	}
	// Try to find existing key
	for i := 0; i < len(node.Content); i += 2 {
		if i+1 >= len(node.Content) {
			break
		}
		if node.Content[i].Value == key {
			node.Content[i+1] = value
			return
		}
	}
	// Key not found, append
	keyNode := &yaml.Node{
		Kind:  yaml.ScalarNode,
		Value: key,
	}
	node.Content = append(node.Content, keyNode, value)
}

// GetMapKeys returns all keys from a mapping node in order
func (h *NodeHelper) GetMapKeys(node *yaml.Node) []string {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	keys := make([]string, 0, len(node.Content)/2)
	for i := 0; i < len(node.Content); i += 2 {
		if i+1 >= len(node.Content) {
			break
		}
		keys = append(keys, node.Content[i].Value)
	}
	return keys
}

// IterateMap iterates over a mapping node, calling fn for each key-value pair
func (h *NodeHelper) IterateMap(node *yaml.Node, fn func(key string, value *yaml.Node) error) error {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i < len(node.Content); i += 2 {
		if i+1 >= len(node.Content) {
			break
		}
		if err := fn(node.Content[i].Value, node.Content[i+1]); err != nil {
			return err
		}
	}
	return nil
}

// NodeToMap converts yaml.Node to map[string]interface{} (loses order)
func (h *NodeHelper) NodeToMap(node *yaml.Node) map[string]interface{} {
	if node == nil {
		return nil
	}
	// Handle document node
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		return h.NodeToMap(node.Content[0])
	}
	if node.Kind != yaml.MappingNode {
		return nil
	}
	result := make(map[string]interface{}, len(node.Content)/2)
	for i := 0; i < len(node.Content); i += 2 {
		if i+1 >= len(node.Content) {
			break
		}
		key := node.Content[i].Value
		result[key] = h.NodeToInterface(node.Content[i+1])
	}
	return result
}

// NodeToInterface converts yaml.Node to interface{}
func (h *NodeHelper) NodeToInterface(node *yaml.Node) interface{} {
	if node == nil {
		return nil
	}
	switch node.Kind {
	case yaml.DocumentNode:
		if len(node.Content) > 0 {
			return h.NodeToInterface(node.Content[0])
		}
		return nil
	case yaml.MappingNode:
		result := make(map[string]interface{}, len(node.Content)/2)
		for i := 0; i < len(node.Content); i += 2 {
			if i+1 >= len(node.Content) {
				break
			}
			key := node.Content[i].Value
			result[key] = h.NodeToInterface(node.Content[i+1])
		}
		return result
	case yaml.SequenceNode:
		result := make([]interface{}, 0, len(node.Content))
		for _, item := range node.Content {
			result = append(result, h.NodeToInterface(item))
		}
		return result
	case yaml.ScalarNode:
		var v interface{}
		if err := node.Decode(&v); err == nil {
			return v
		}
		return node.Value
	default:
		return node.Value
	}
}

// MapToNode converts map[string]interface{} to yaml.Node
func (h *NodeHelper) MapToNode(m map[string]interface{}) *yaml.Node {
	node := &yaml.Node{
		Kind: yaml.MappingNode,
	}
	for key, value := range m {
		keyNode := &yaml.Node{
			Kind:  yaml.ScalarNode,
			Value: key,
		}
		valueNode := h.InterfaceToNode(value)
		node.Content = append(node.Content, keyNode, valueNode)
	}
	return node
}

// InterfaceToNode converts interface{} to yaml.Node
func (h *NodeHelper) InterfaceToNode(v interface{}) *yaml.Node {
	if v == nil {
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null"}
	}
	switch val := v.(type) {
	case map[string]interface{}:
		return h.MapToNode(val)
	case []interface{}:
		node := &yaml.Node{Kind: yaml.SequenceNode}
		for _, item := range val {
			node.Content = append(node.Content, h.InterfaceToNode(item))
		}
		return node
	case string:
		return &yaml.Node{Kind: yaml.ScalarNode, Value: val}
	case int:
		return &yaml.Node{Kind: yaml.ScalarNode, Value: fmt.Sprintf("%d", val), Tag: "!!int"}
	case int64:
		return &yaml.Node{Kind: yaml.ScalarNode, Value: fmt.Sprintf("%d", val), Tag: "!!int"}
	case float64:
		return &yaml.Node{Kind: yaml.ScalarNode, Value: fmt.Sprintf("%v", val), Tag: "!!float"}
	case bool:
		return &yaml.Node{Kind: yaml.ScalarNode, Value: fmt.Sprintf("%t", val), Tag: "!!bool"}
	default:
		node := &yaml.Node{}
		_ = node.Encode(v)
		return node
	}
}

// CloneNode creates a deep copy of a yaml.Node
func (h *NodeHelper) CloneNode(node *yaml.Node) *yaml.Node {
	if node == nil {
		return nil
	}
	clone := &yaml.Node{
		Kind:        node.Kind,
		Style:       node.Style,
		Tag:         node.Tag,
		Value:       node.Value,
		Anchor:      node.Anchor,
		Alias:       node.Alias,
		HeadComment: node.HeadComment,
		LineComment: node.LineComment,
		FootComment: node.FootComment,
		Line:        node.Line,
		Column:      node.Column,
	}
	if len(node.Content) > 0 {
		clone.Content = make([]*yaml.Node, len(node.Content))
		for i, child := range node.Content {
			clone.Content[i] = h.CloneNode(child)
		}
	}
	return clone
}

// DeleteMapKey removes a key from a mapping node
func (h *NodeHelper) DeleteMapKey(node *yaml.Node, key string) {
	if node == nil || node.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i < len(node.Content); i += 2 {
		if i+1 >= len(node.Content) {
			break
		}
		if node.Content[i].Value == key {
			// Remove key and value
			node.Content = append(node.Content[:i], node.Content[i+2:]...)
			return
		}
	}
}

// HasMapKey checks if a mapping node has a key
func (h *NodeHelper) HasMapKey(node *yaml.Node, key string) bool {
	return h.GetMapValue(node, key) != nil
}

// GetStringValue gets a string value from a scalar node
func (h *NodeHelper) GetStringValue(node *yaml.Node) string {
	if node == nil || node.Kind != yaml.ScalarNode {
		return ""
	}
	return node.Value
}

// IsRef checks if a node is a $ref
func (h *NodeHelper) IsRef(node *yaml.Node) bool {
	if node == nil || node.Kind != yaml.MappingNode {
		return false
	}
	return h.HasMapKey(node, "$ref")
}

// GetRef gets the $ref value from a node
func (h *NodeHelper) GetRef(node *yaml.Node) string {
	if node == nil || node.Kind != yaml.MappingNode {
		return ""
	}
	refNode := h.GetMapValue(node, "$ref")
	if refNode == nil {
		return ""
	}
	return h.GetStringValue(refNode)
}

// SetRef sets the $ref value on a node
func (h *NodeHelper) SetRef(node *yaml.Node, ref string) {
	refNode := &yaml.Node{
		Kind:  yaml.ScalarNode,
		Value: ref,
	}
	h.SetMapValue(node, "$ref", refNode)
}

// CreateRefNode creates a new node with just a $ref
func (h *NodeHelper) CreateRefNode(ref string) *yaml.Node {
	return &yaml.Node{
		Kind: yaml.MappingNode,
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Value: "$ref"},
			{Kind: yaml.ScalarNode, Value: ref},
		},
	}
}

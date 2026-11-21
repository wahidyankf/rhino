package config

import "gopkg.in/yaml.v3"

// LineOf returns the line of the mapping key or gate entry that a path names,
// or 0 when the document holds no such node. A segment names a key of a
// mapping; inside a sequence it names the item whose `id` is that segment, so
// `gates`, `entries`, and a gate identifier name one gate entry.
func (d *Document) LineOf(segments ...string) int {
	if d.root == nil || len(d.root.Content) == 0 {
		return 0
	}
	node := d.root.Content[0]
	for _, segment := range segments {
		node = childNode(node, segment)
		if node == nil {
			return 0
		}
	}
	return node.Line
}

// childNode returns the value a segment names inside one node, or nil.
func childNode(node *yaml.Node, segment string) *yaml.Node {
	switch node.Kind {
	case yaml.MappingNode:
		return mappingValue(node, segment)
	case yaml.SequenceNode:
		for _, item := range node.Content {
			if id := mappingValue(item, "id"); id != nil && id.Value == segment {
				return item
			}
		}
	}
	return nil
}

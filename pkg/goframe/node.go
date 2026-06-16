package goframe

// Node is a value that can be rendered into the DOM.
type Node interface {
	isNode()
}

// VNode describes an HTML element and its children.
type VNode struct {
	Tag      string
	Props    Props
	Children []Node
}

func (VNode) isNode() {}

// TextNode describes a DOM text node.
type TextNode struct {
	Value string
}

func (TextNode) isNode() {}

// FragmentNode groups children without creating an HTML element.
type FragmentNode struct {
	Children []Node
}

func (FragmentNode) isNode() {}

// EmptyNode represents an intentionally empty render result.
type EmptyNode struct{}

func (EmptyNode) isNode() {}

// KeyedNode associates a stable reconciliation identity with a node.
type KeyedNode struct {
	Key  string
	Node Node
}

func (KeyedNode) isNode() {}

// El creates an element node.
func El(tag string, props Props, children ...Node) Node {
	return VNode{
		Tag:      tag,
		Props:    props,
		Children: children,
	}
}

// Text creates a text node.
func Text(value string) Node {
	return TextNode{Value: value}
}

// Fragment groups children without adding a wrapper element.
func Fragment(children ...Node) Node {
	return FragmentNode{Children: children}
}

// Empty creates a renderable node that produces no DOM content.
func Empty() Node {
	return EmptyNode{}
}

// Child converts a GOX expression into a renderable node. Node and []Node
// values keep their structure; supported primitive values become text.
func Child(value any) Node {
	switch value := value.(type) {
	case nil:
		return Empty()
	case Node:
		if value == nil {
			return Empty()
		}
		return value
	case []Node:
		return Fragment(value...)
	default:
		return Text(ToString(value))
	}
}

// If renders node when condition is true and otherwise returns an empty node.
func If(condition bool, node Node) Node {
	if condition {
		return Child(node)
	}
	return Empty()
}

// IfElse selects one of two nodes.
func IfElse(condition bool, thenNode Node, elseNode Node) Node {
	if condition {
		return Child(thenNode)
	}
	return Child(elseNode)
}

// For maps items into renderable nodes.
func For[T any](items []T, render func(item T) Node) []Node {
	nodes := make([]Node, 0, len(items))
	for _, item := range items {
		nodes = append(nodes, Child(render(item)))
	}
	return nodes
}

// ForIndexed maps items into renderable nodes and provides each item index.
func ForIndexed[T any](items []T, render func(index int, item T) Node) []Node {
	nodes := make([]Node, 0, len(items))
	for index, item := range items {
		nodes = append(nodes, Child(render(index, item)))
	}
	return nodes
}

// Key associates a stable reconciliation identity with node.
func Key(key string, node Node) Node {
	return KeyedNode{Key: key, Node: Child(node)}
}

// WithKey is the node-first form of Key.
func WithKey(node Node, key string) Node {
	return Key(key, node)
}

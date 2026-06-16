package gox

// Node is one node in a parsed GOX element tree.
type Node interface {
	isGOXNode()
}

// Element is an HTML-like GOX element.
type Element struct {
	Tag        string
	Attributes []Attribute
	Children   []Node
}

func (*Element) isGOXNode() {}

// Fragment groups children without an HTML wrapper.
type Fragment struct {
	Children []Node
}

func (*Fragment) isGOXNode() {}

// Text is literal text between tags.
type Text struct {
	Value string
}

func (*Text) isGOXNode() {}

// Expression is Go code enclosed in braces.
type Expression struct {
	Code string
}

func (*Expression) isGOXNode() {}

// Attribute is one element property.
type Attribute struct {
	Name  string
	Value AttributeValue
}

// AttributeValue is a string, Go expression, or boolean attribute.
type AttributeValue interface {
	isAttributeValue()
}

// StringValue is a quoted attribute value.
type StringValue struct {
	Value string
}

func (StringValue) isAttributeValue() {}

// ExpressionValue is a Go expression used as an attribute value.
type ExpressionValue struct {
	Code string
}

func (ExpressionValue) isAttributeValue() {}

// BoolValue represents an attribute without an explicit value.
type BoolValue struct{}

func (BoolValue) isAttributeValue() {}

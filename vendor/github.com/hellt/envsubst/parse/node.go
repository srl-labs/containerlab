package parse

import (
	"fmt"
)

type Node interface {
	Type() NodeType
	String() (string, error)
}

// NodeType identifies the type of a node.
type NodeType int

// Type returns itself and provides an easy default implementation
// for embedding in a Node. Embedded in all non-trivial Nodes.
func (t NodeType) Type() NodeType {
	return t
}

const (
	NodeText NodeType = iota
	NodeSubstitution
	NodeVariable
)

type TextNode struct {
	NodeType
	Text string
}

func NewText(text string) *TextNode {
	return &TextNode{NodeText, text}
}

func (t *TextNode) String() (string, error) {
	return t.Text, nil
}

type VariableNode struct {
	NodeType
	Ident    string
	Env      Env
	Restrict *Restrictions
}

func NewVariable(ident string, env Env, restrict *Restrictions) *VariableNode {
	return &VariableNode{NodeVariable, ident, env, restrict}
}

func (t *VariableNode) String() (string, error) {
	if err := t.validateNoUnset(); err != nil {
		return "", err
	}

	return t.validateNoEmpty(t.Env.Get(t.Ident))
}

func (t *VariableNode) isSet() bool {
	return t.Env.Has(t.Ident)
}

func (t *VariableNode) validateNoUnset() error {
	if t.Restrict.NoUnset && !t.isSet() {
		return fmt.Errorf("variable ${%s} not set", t.Ident)
	}
	return nil
}

func (t *VariableNode) validateNoEmpty(value string) (string, error) {
	if t.Restrict.NoReplace && len(value) < 1 {
		return fmt.Sprintf("$%s", t.Ident), nil
	}
	if t.Restrict.NoEmpty && value == "" && t.isSet() {
		return "", fmt.Errorf("variable ${%s} set but empty", t.Ident)
	}
	return value, nil
}

type SubstitutionNode struct {
	NodeType
	ExpType  itemType
	Variable *VariableNode
	Default  Node // Default could be variable or text
}

func (t *SubstitutionNode) String() (string, error) {
	if t.ExpType >= itemPlus && t.Default != nil {
		switch t.ExpType {
		case itemColonDash, itemColonEquals:
			s, _ := t.Variable.String()
			// if default is set and the returned string equals the var name, apply the default
			if t.Default != nil && s == fmt.Sprintf("$%s", t.Variable.Ident) {
				return t.Default.String()
			}
			if s != "" {
				return s, nil
			}
			return t.Default.String()
		case itemPlus, itemColonPlus:
			if t.Variable.isSet() {
				return t.Default.String()
			}
			return "", nil
		default:
			if !t.Variable.isSet() {
				return t.Default.String()
			}
		}
	}
	return t.Variable.String()
}

package parser

import (
	"context"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/python"
)

type Field struct {
	Model string
	Name  string
}

// parseCtxFn is the function used to parse Python source into a tree.
// It is a var so tests can inject a failing version to cover error paths.
var parseCtxFn = func(p *sitter.Parser, ctx context.Context, oldTree *sitter.Tree, src []byte) (*sitter.Tree, error) {
	return p.ParseCtx(ctx, oldTree, src)
}

func ParseModels(src []byte) ([]Field, error) {
	p := sitter.NewParser()
	p.SetLanguage(python.GetLanguage())

	tree, err := parseCtxFn(p, context.Background(), nil, src)
	if err != nil {
		return nil, err
	}

	var fields []Field
	root := tree.RootNode()

	for i := 0; i < int(root.ChildCount()); i++ {
		node := root.Child(i)
		if node.Type() != "class_definition" {
			continue
		}
		if !isModelClass(node, src) {
			continue
		}
		className := node.ChildByFieldName("name").Content(src)
		for _, name := range extractFields(node, src) {
			fields = append(fields, Field{Model: className, Name: name})
		}
	}

	return fields, nil
}

// isModelClass returns true when a class inherits from a name ending in "Model".
// This covers models.Model, AbstractModel, TimeStampedModel, etc.
func isModelClass(node *sitter.Node, src []byte) bool {
	superclasses := node.ChildByFieldName("superclasses")
	if superclasses == nil {
		return false
	}
	for i := 0; i < int(superclasses.ChildCount()); i++ {
		child := superclasses.Child(i)
		var ident string
		switch child.Type() {
		case "identifier":
			ident = child.Content(src)
		case "attribute":
			attr := child.ChildByFieldName("attribute")
			if attr != nil {
				ident = attr.Content(src)
			}
		}
		if strings.HasSuffix(ident, "Model") {
			return true
		}
	}
	return false
}

// extractFields returns the names of Django field assignments in a class body.
// It matches assignments whose right-hand side references something ending in "Field"
// or starts with "models." — the heuristic that covers all built-in Django fields
// and most third-party ones.
func extractFields(classNode *sitter.Node, src []byte) []string {
	body := classNode.ChildByFieldName("body")
	if body == nil {
		return nil
	}

	var names []string
	for i := 0; i < int(body.ChildCount()); i++ {
		stmt := body.Child(i)
		if stmt.Type() != "expression_statement" {
			continue
		}
		assign := stmt.Child(0)
		if assign == nil || assign.Type() != "assignment" {
			continue
		}
		left := assign.ChildByFieldName("left")
		right := assign.ChildByFieldName("right")
		if left == nil || left.Type() != "identifier" {
			continue
		}
		if isDjangoField(right, src) {
			names = append(names, left.Content(src))
		}
	}
	return names
}

func isDjangoField(node *sitter.Node, src []byte) bool {
	if node == nil {
		return false
	}
	// Unwrap call expressions to inspect the function being called.
	if node.Type() == "call" {
		fn := node.ChildByFieldName("function")
		return isDjangoField(fn, src)
	}
	// attribute access: models.CharField, models.ForeignKey, etc.
	// Anything under models.* is a Django field.
	// For third-party fields not under models., fall back to the "Field" suffix.
	if node.Type() == "attribute" {
		obj := node.ChildByFieldName("object")
		if obj != nil && obj.Content(src) == "models" {
			return true
		}
		attr := node.ChildByFieldName("attribute")
		if attr != nil && strings.HasSuffix(attr.Content(src), "Field") {
			return true
		}
		return false
	}
	// bare identifier: CharField (when imported directly)
	if node.Type() == "identifier" {
		return strings.HasSuffix(node.Content(src), "Field")
	}
	return false
}

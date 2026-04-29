package parser

import (
	"context"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/python"

	"github.com/shinagawa-web/colref/internal/orm"
)

// Field is a type alias for orm.Field for backward compatibility.
type Field = orm.Field

// DjangoParser implements orm.SchemaParser for Django/Python models.
type DjangoParser struct{}

// ParseSchema implements orm.SchemaParser.
func (DjangoParser) ParseSchema(src []byte) ([]orm.Field, error) {
	return ParseModels(src)
}

// parseCtxFn is the function used to parse Python source into a tree.
// It is a var so tests can inject a failing version to cover error paths.
var parseCtxFn = func(p *sitter.Parser, ctx context.Context, oldTree *sitter.Tree, src []byte) (*sitter.Tree, error) {
	return p.ParseCtx(ctx, oldTree, src)
}

// classEntry holds the info extracted from a class definition for model-set
// computation. It is not exported; callers use BuildModelSet.
type classEntry struct {
	name           string
	isDirectDjango bool     // inherits directly from models.Model or bare Model
	superNames     []string // bare-identifier superclass names for transitive lookup
	node           *sitter.Node
	src            []byte
}

// extractClassEntries returns all class entries from a parsed tree root node.
func extractClassEntries(root *sitter.Node, src []byte) []classEntry {
	var entries []classEntry
	for i := 0; i < int(root.ChildCount()); i++ {
		node := root.Child(i)
		if node.Type() != "class_definition" {
			continue
		}
		name := node.ChildByFieldName("name").Content(src)
		superclasses := node.ChildByFieldName("superclasses")

		var isDirectDjango bool
		var superNames []string

		if superclasses != nil {
			for j := 0; j < int(superclasses.ChildCount()); j++ {
				child := superclasses.Child(j)
				switch child.Type() {
				case "identifier":
					n := child.Content(src)
					superNames = append(superNames, n)
					// bare "Model" covers: from django.db.models import Model
					if n == "Model" {
						isDirectDjango = true
					}
				case "attribute":
					obj := child.ChildByFieldName("object")
					attr := child.ChildByFieldName("attribute")
					if obj != nil && attr != nil && obj.Content(src) == "models" && attr.Content(src) == "Model" {
						isDirectDjango = true
					}
					// Do not add attr to superNames; attribute-style bases like
					// models.Model are only used for seed detection, not
					// transitive lookup.
				}
			}
		}

		entries = append(entries, classEntry{
			name:           name,
			isDirectDjango: isDirectDjango,
			superNames:     superNames,
			node:           node,
			src:            src,
		})
	}
	return entries
}

// computeModelSet builds the Django-model set using transitive closure.
// Seeds are classes that directly inherit from models.Model.
func computeModelSet(all []classEntry) map[string]bool {
	modelSet := make(map[string]bool)
	for _, c := range all {
		if c.isDirectDjango {
			modelSet[c.name] = true
		}
	}
	for {
		added := 0
		for _, c := range all {
			if modelSet[c.name] {
				continue
			}
			for _, s := range c.superNames {
				if modelSet[s] {
					modelSet[c.name] = true
					added++
					break
				}
			}
		}
		if added == 0 {
			break
		}
	}
	return modelSet
}

// BuildModelSet parses all provided Python sources and returns the set of
// class names that are Django models, using transitive closure across all files.
func BuildModelSet(sources [][]byte) (map[string]bool, error) {
	var all []classEntry
	for _, src := range sources {
		p := sitter.NewParser()
		p.SetLanguage(python.GetLanguage())
		tree, err := parseCtxFn(p, context.Background(), nil, src)
		if err != nil {
			return nil, err
		}
		all = append(all, extractClassEntries(tree.RootNode(), src)...)
	}
	return computeModelSet(all), nil
}

// ParseModelsWithSet extracts fields from classes in src that appear in modelSet.
func ParseModelsWithSet(src []byte, modelSet map[string]bool) ([]Field, error) {
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
		className := node.ChildByFieldName("name").Content(src)
		if !modelSet[className] {
			continue
		}
		for _, name := range extractFields(node, src) {
			fields = append(fields, Field{Model: className, Name: name})
		}
	}
	return fields, nil
}

// ParseModels parses a single Python source file and returns fields for all
// detected Django models, using within-file transitive closure.
func ParseModels(src []byte) ([]Field, error) {
	p := sitter.NewParser()
	p.SetLanguage(python.GetLanguage())
	tree, err := parseCtxFn(p, context.Background(), nil, src)
	if err != nil {
		return nil, err
	}
	root := tree.RootNode()
	entries := extractClassEntries(root, src)
	modelSet := computeModelSet(entries)

	var fields []Field
	for i := 0; i < int(root.ChildCount()); i++ {
		node := root.Child(i)
		if node.Type() != "class_definition" {
			continue
		}
		className := node.ChildByFieldName("name").Content(src)
		if !modelSet[className] {
			continue
		}
		for _, name := range extractFields(node, src) {
			fields = append(fields, Field{Model: className, Name: name})
		}
	}
	return fields, nil
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

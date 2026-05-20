package parser

import (
	"context"
	"strings"
	"unicode"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/ruby"

	"github.com/shinagawa-web/colref/internal/orm"
)

// RailsParser implements orm.SchemaParser for Rails db/schema.rb files.
type RailsParser struct{}

// ParseSchema implements orm.SchemaParser.
func (RailsParser) ParseSchema(src []byte) ([]orm.Field, error) {
	return ParseSchemaRb(src)
}

// parseCtxFnRuby is the function used to parse Ruby source into a tree.
// It is a var so tests can inject a failing version to cover error paths.
var parseCtxFnRuby = func(p *sitter.Parser, ctx context.Context, oldTree *sitter.Tree, src []byte) (*sitter.Tree, error) {
	return p.ParseCtx(ctx, oldTree, src)
}

// ParseSchemaRb parses a Rails db/schema.rb file and returns all column
// definitions as Fields. The Model name is inferred from the table name via
// a simple singularize+CamelCase heuristic; self.table_name overrides are
// not handled (v0.1 limitation).
func ParseSchemaRb(src []byte) ([]Field, error) {
	p := sitter.NewParser()
	p.SetLanguage(ruby.GetLanguage())

	tree, err := parseCtxFnRuby(p, context.Background(), nil, src)
	if err != nil {
		return nil, err
	}

	var fields []Field
	walkSchemaNode(tree.RootNode(), src, &fields)
	return fields, nil
}

// walkSchemaNode recursively walks the AST looking for create_table calls.
func walkSchemaNode(node *sitter.Node, src []byte, fields *[]Field) {
	if node.Type() == "call" {
		method := node.ChildByFieldName("method")
		receiver := node.ChildByFieldName("receiver")
		if receiver == nil && method != nil && method.Content(src) == "create_table" {
			extractTableFields(node, src, fields)
			return
		}
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		walkSchemaNode(node.Child(i), src, fields)
	}
}

// extractTableFields extracts column names from a single create_table block.
func extractTableFields(callNode *sitter.Node, src []byte, fields *[]Field) {
	args := callNode.ChildByFieldName("arguments")
	if args == nil {
		return
	}
	tableName := firstStringArg(args, src)
	if tableName == "" {
		return
	}
	modelName := tableToModel(tableName)

	doBlock := findChild(callNode, "do_block")
	if doBlock == nil {
		return
	}
	body := findChild(doBlock, "body_statement")
	if body == nil {
		return
	}

	for i := 0; i < int(body.ChildCount()); i++ {
		stmt := body.Child(i)
		if stmt.Type() != "call" {
			continue
		}
		colArgs := stmt.ChildByFieldName("arguments")
		if colArgs == nil {
			continue
		}
		colName := firstStringArg(colArgs, src)
		if colName == "" {
			continue
		}
		*fields = append(*fields, Field{Model: modelName, Name: colName})
	}
}

// firstStringArg returns the content of the first string argument in an
// argument_list node, or "" if no string argument is present.
func firstStringArg(argList *sitter.Node, src []byte) string {
	for i := 0; i < int(argList.ChildCount()); i++ {
		child := argList.Child(i)
		if child.Type() == "string" {
			for j := 0; j < int(child.ChildCount()); j++ {
				c := child.Child(j)
				if c.Type() == "string_content" {
					return c.Content(src)
				}
			}
		}
	}
	return ""
}

// findChild returns the first direct child of node with the given type.
func findChild(node *sitter.Node, typ string) *sitter.Node {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == typ {
			return child
		}
	}
	return nil
}

// tableToModel converts a Rails table name (e.g. "order_items") to a model
// name (e.g. "OrderItem") using a simple singularize+CamelCase heuristic.
// Irregular plurals (e.g. people→Person) are not handled (v0.1 limitation).
func tableToModel(table string) string {
	words := strings.Split(table, "_")
	for i, w := range words {
		if i == len(words)-1 {
			w = singularize(w)
		}
		words[i] = capitalize(w)
	}
	return strings.Join(words, "")
}

func singularize(word string) string {
	switch {
	case strings.HasSuffix(word, "ies") && len(word) > 3:
		return word[:len(word)-3] + "y"
	case strings.HasSuffix(word, "sses"):
		return word[:len(word)-2]
	case strings.HasSuffix(word, "xes"), strings.HasSuffix(word, "zes"),
		strings.HasSuffix(word, "ches"), strings.HasSuffix(word, "shes"),
		strings.HasSuffix(word, "ses"):
		return word[:len(word)-2]
	case strings.HasSuffix(word, "s") && len(word) > 1:
		return word[:len(word)-1]
	default:
		return word
	}
}

func capitalize(s string) string {
	if s == "" {
		return ""
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

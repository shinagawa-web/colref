package refs

import (
	"context"
	"sort"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/python"
	gosql "github.com/smacker/go-tree-sitter/sql"

	"github.com/shinagawa-web/colref/internal/orm"
)

// SkipDirs is the set of directory names that are never scanned.
// Exported so CLI code that walks for models.py can apply the same rules.
var SkipDirs = map[string]bool{
	"__pycache__":  true,
	"venv":         true,
	"migrations":   true,
	"node_modules": true,
}

// positionalStringMethods are Django ORM methods whose string positional arguments
// name a model field.
var positionalStringMethods = map[string]bool{
	"values": true, "values_list": true, "defer": true, "only": true,
	"order_by": true, "select_related": true, "prefetch_related": true,
	"latest": true, "earliest": true, "distinct": true,
}

// firstArgStringFunctions are Django expression/aggregate functions whose first
// positional string argument is a field name (same convention as F()).
var firstArgStringFunctions = map[string]bool{
	"F":   true,
	"Max": true, "Min": true, "Avg": true, "Sum": true, "Count": true,
	"StdDev": true, "Variance": true,
	"Coalesce": true, "Concat": true, "Greatest": true, "Least": true,
	"NullIf": true, "OuterRef": true, "Subquery": true,
}

// keywordArgMethods are Django ORM methods (and Q) whose keyword argument names
// refer to model fields (possibly with lookup suffixes like __icontains).
var keywordArgMethods = map[string]bool{
	"filter": true, "exclude": true, "annotate": true, "Q": true,
	"get": true, "create": true, "update": true, "get_or_create": true,
}

// sqlMethods are Python methods whose first positional string argument is raw SQL.
var sqlMethods = map[string]bool{
	"raw": true, "execute": true,
}

// sqlDMLPrefixes are SQL DML keywords a raw SQL string must begin with
// (case-insensitive) before we bother parsing it. This avoids feeding
// arbitrary strings (e.g. log messages) to the SQL grammar.
var sqlDMLPrefixes = []string{"SELECT", "INSERT", "UPDATE", "DELETE", "WITH"}

// PythonScanner implements orm.ReferenceScanner for Python codebases.
type PythonScanner struct{}

// Scan implements orm.ReferenceScanner.
func (PythonScanner) Scan(dir, fieldName string) ([]orm.Reference, int, error) {
	return ScanDjango(dir, fieldName)
}

// SkipDirs implements orm.ReferenceScanner, returning a defensive copy.
func (PythonScanner) SkipDirs() map[string]bool {
	copy := make(map[string]bool, len(SkipDirs))
	for k, v := range SkipDirs {
		copy[k] = v
	}
	return copy
}

// ScanDjango combines attribute-access and string-based ORM scanning for
// Django projects. Results are merged and sorted by (File, Line).
func ScanDjango(dir, fieldName string) ([]Reference, int, error) {
	attrRefs, count, err := Scan(dir, fieldName)
	if err != nil {
		return nil, 0, err
	}
	strRefs, _, err := ScanStringRefs(dir, fieldName)
	if err != nil {
		return nil, 0, err
	}
	refs := append(attrRefs, strRefs...)
	sort.Slice(refs, func(i, j int) bool {
		if refs[i].File != refs[j].File {
			return refs[i].File < refs[j].File
		}
		return refs[i].Line < refs[j].Line
	})
	return refs, count, nil
}

// Scan walks dir and returns every attribute-access node whose attribute name
// equals fieldName, along with the total number of .py files examined.
func Scan(dir, fieldName string) ([]Reference, int, error) {
	return scanFiles(dir, fieldName, map[string]func([]byte) []byte{".py": nil}, python.GetLanguage(), walkNode, SkipDirs)
}

// ScanStringRefs walks dir and returns every string-based Django ORM reference
// to fieldName: positional string args in values/defer/only/etc., keyword arg
// names in filter/exclude/Q, and the first arg of F(). Each reference's Text
// is prefixed with "[string] ".
func ScanStringRefs(dir, fieldName string) ([]Reference, int, error) {
	return scanFiles(dir, fieldName, map[string]func([]byte) []byte{".py": nil}, python.GetLanguage(), walkNodeStringRefs, SkipDirs)
}

// walkNodeStringRefs finds string-based Django ORM references to fieldName.
func walkNodeStringRefs(node *sitter.Node, src []byte, lines [][]byte, fieldName, file string, refs *[]Reference) {
	if node.Type() == "call" {
		fn := node.ChildByFieldName("function")
		if fn != nil {
			methodName := callMethodName(fn, src)
			args := node.ChildByFieldName("arguments")
			if args != nil {
				switch {
				case firstArgStringFunctions[methodName]:
					for i := 0; i < int(args.ChildCount()); i++ {
						child := args.Child(i)
						if child.Type() == "string" {
							if stringContent(child, src) == fieldName {
								addStringRef(child, lines, file, refs)
							}
							break
						}
					}
				case positionalStringMethods[methodName]:
					for i := 0; i < int(args.ChildCount()); i++ {
						child := args.Child(i)
						if child.Type() == "string" {
							content := stringContent(child, src)
							if methodName == "order_by" {
								content = strings.TrimPrefix(content, "-")
							}
							if content == fieldName {
								addStringRef(child, lines, file, refs)
							}
						}
					}
				case keywordArgMethods[methodName]:
					for i := 0; i < int(args.ChildCount()); i++ {
						child := args.Child(i)
						if child.Type() == "keyword_argument" {
							nameNode := child.ChildByFieldName("name")
							if nameNode != nil {
								kwName := nameNode.Content(src)
								if idx := strings.Index(kwName, "__"); idx >= 0 {
									kwName = kwName[:idx]
								}
								if kwName == fieldName {
									addStringRef(nameNode, lines, file, refs)
								}
							}
						}
					}
				case sqlMethods[methodName]:
					for i := 0; i < int(args.ChildCount()); i++ {
						child := args.Child(i)
						if child.Type() == "string" {
							content := stringContent(child, src)
							if isSQLString(content) && sqlContainsField([]byte(content), fieldName) {
								addSqlRef(child, lines, file, refs)
							}
							break
						}
					}
				}
			}
		}
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		walkNodeStringRefs(node.Child(i), src, lines, fieldName, file, refs)
	}
}

// callMethodName extracts the method name from a function node (attribute or identifier).
func callMethodName(fn *sitter.Node, src []byte) string {
	switch fn.Type() {
	case "attribute":
		attr := fn.ChildByFieldName("attribute")
		if attr != nil {
			return attr.Content(src)
		}
	case "identifier":
		return fn.Content(src)
	}
	return ""
}

// stringContent returns the string_content of a string node, or "".
func stringContent(node *sitter.Node, src []byte) string {
	for i := 0; i < int(node.ChildCount()); i++ {
		c := node.Child(i)
		if c.Type() == "string_content" {
			return c.Content(src)
		}
	}
	return ""
}

// addStringRef appends a [string]-labeled Reference using the node's source row.
func addStringRef(node *sitter.Node, lines [][]byte, file string, refs *[]Reference) {
	row := int(node.StartPoint().Row)
	*refs = append(*refs, Reference{
		File: file,
		Line: row + 1,
		Text: "[string] " + strings.TrimSpace(lineAt(lines, row)),
	})
}

// addSqlRef appends a [sql ref]-labeled Reference using the node's source row.
func addSqlRef(node *sitter.Node, lines [][]byte, file string, refs *[]Reference) {
	row := int(node.StartPoint().Row)
	*refs = append(*refs, Reference{
		File: file,
		Line: row + 1,
		Text: "[sql ref] " + strings.TrimSpace(lineAt(lines, row)),
	})
}

// isSQLString reports whether s looks like a SQL DML statement by checking
// that its trimmed prefix matches a known DML keyword.
func isSQLString(s string) bool {
	upper := strings.ToUpper(strings.TrimSpace(s))
	for _, kw := range sqlDMLPrefixes {
		if strings.HasPrefix(upper, kw) {
			return true
		}
	}
	return false
}

// sqlParseCtxFn is the function used to parse SQL into a tree.
// It is a var so tests can inject a failing version to cover error paths.
var sqlParseCtxFn = func(p *sitter.Parser, src []byte) (*sitter.Tree, error) {
	return p.ParseCtx(context.Background(), nil, src)
}

// sqlContainsField reports whether fieldName appears as a SQL field identifier
// in sqlSrc. The SQL is parsed with the tree-sitter SQL grammar; field nodes
// whose identifier child matches fieldName trigger a true return.
// Note: single-letter field names risk false positives from %s placeholders,
// which the SQL grammar parses as (ERROR "%")(field "s").
func sqlContainsField(sqlSrc []byte, fieldName string) bool {
	p := sitter.NewParser()
	p.SetLanguage(gosql.GetLanguage())
	tree, err := sqlParseCtxFn(p, sqlSrc)
	if err != nil || tree == nil {
		return false
	}
	return walkSQLFieldNode(tree.RootNode(), sqlSrc, fieldName)
}

// walkSQLFieldNode recursively walks a SQL AST and returns true if any field
// node contains an identifier child that equals fieldName.
func walkSQLFieldNode(node *sitter.Node, src []byte, fieldName string) bool {
	if node.Type() == "field" {
		for i := 0; i < int(node.ChildCount()); i++ {
			c := node.Child(i)
			if c.Type() == "identifier" && c.Content(src) == fieldName {
				return true
			}
		}
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		if walkSQLFieldNode(node.Child(i), src, fieldName) {
			return true
		}
	}
	return false
}

func walkNode(node *sitter.Node, src []byte, lines [][]byte, fieldName, file string, refs *[]Reference) {
	if node.Type() == "attribute" {
		attr := node.ChildByFieldName("attribute")
		if attr != nil && attr.Content(src) == fieldName {
			attrRow := int(attr.StartPoint().Row)
			text := node.Content(src)
			if int(node.StartPoint().Row) != attrRow {
				text = strings.TrimSpace(lineAt(lines, attrRow))
			}
			*refs = append(*refs, Reference{
				File: file,
				Line: attrRow + 1,
				Text: text,
			})
			// The object subtree cannot itself end in the same attribute name
			// unless it is a coincidental deeper match, so keep recursing to
			// catch patterns like a.email.email (rare but valid).
		}
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		walkNode(node.Child(i), src, lines, fieldName, file, refs)
	}
}

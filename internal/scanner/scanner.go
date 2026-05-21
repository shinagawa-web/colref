package scanner

import (
	"bytes"
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/python"
	"github.com/smacker/go-tree-sitter/ruby"

	"github.com/shinagawa-web/colref/internal/orm"
)

// Reference is a type alias for orm.Reference for backward compatibility.
type Reference = orm.Reference

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

// RubyScanner implements orm.ReferenceScanner for Ruby codebases.
type RubyScanner struct{}

// Scan implements orm.ReferenceScanner.
func (RubyScanner) Scan(dir, fieldName string) ([]orm.Reference, int, error) {
	return ScanRuby(dir, fieldName)
}

// SkipDirs implements orm.ReferenceScanner, returning a defensive copy.
func (RubyScanner) SkipDirs() map[string]bool {
	copy := make(map[string]bool, len(RubySkipDirs))
	for k, v := range RubySkipDirs {
		copy[k] = v
	}
	return copy
}

// SkipDirs is the set of directory names that are never scanned.
// Exported so CLI code that walks for models.py can apply the same rules.
var SkipDirs = map[string]bool{
	"__pycache__":  true,
	"venv":         true,
	"migrations":   true,
	"node_modules": true,
}

// RubySkipDirs is the set of directory names skipped when scanning Ruby projects.
var RubySkipDirs = map[string]bool{
	"spec":         true,
	"test":         true,
	"vendor":       true,
	"migrate":      true,
	"node_modules": true,
}

// parseCtxFn is the function used to parse source into a tree.
// It is a var so tests can inject a failing version to cover error paths.
var parseCtxFn = func(p *sitter.Parser, ctx context.Context, oldTree *sitter.Tree, src []byte) (*sitter.Tree, error) {
	return p.ParseCtx(ctx, oldTree, src)
}

// filepathRelFn is the function used to compute relative paths.
// It is a var so tests can inject a failing version to cover error paths.
var filepathRelFn = filepath.Rel

// Scan walks dir and returns every attribute-access node whose attribute name
// equals fieldName, along with the total number of .py files examined.
func Scan(dir, fieldName string) ([]Reference, int, error) {
	return scanFiles(dir, fieldName, map[string]func([]byte) []byte{".py": nil}, python.GetLanguage(), walkNode, SkipDirs)
}

// positionalStringMethods are Django ORM methods whose string positional arguments
// name a model field.
var positionalStringMethods = map[string]bool{
	"values": true, "values_list": true, "defer": true, "only": true,
	"order_by": true, "select_related": true, "prefetch_related": true,
	"latest": true, "earliest": true, "distinct": true,
}

// keywordArgMethods are Django ORM methods (and Q) whose keyword argument names
// refer to model fields (possibly with lookup suffixes like __icontains).
var keywordArgMethods = map[string]bool{
	"filter": true, "exclude": true, "annotate": true, "Q": true,
	"get": true, "create": true, "update": true, "get_or_create": true,
}

// ScanStringRefs walks dir and returns every string-based Django ORM reference
// to fieldName: positional string args in values/defer/only/etc., keyword arg
// names in filter/exclude/Q, and the first arg of F(). Each reference's Text
// is prefixed with "[string] ".
func ScanStringRefs(dir, fieldName string) ([]Reference, int, error) {
	return scanFiles(dir, fieldName, map[string]func([]byte) []byte{".py": nil}, python.GetLanguage(), walkNodeStringRefs, SkipDirs)
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

// ScanRuby walks dir and returns every method-call node whose method name
// equals fieldName, along with the total number of .rb and .erb files examined.
func ScanRuby(dir, fieldName string) ([]Reference, int, error) {
	return scanFiles(dir, fieldName, map[string]func([]byte) []byte{
		".rb":  nil,
		".erb": erbToRuby,
	}, ruby.GetLanguage(), walkNodeRuby, RubySkipDirs)
}

type walkFn func(node *sitter.Node, src []byte, lines [][]byte, fieldName, file string, refs *[]Reference)

// scanFiles walks dir once and processes every file whose extension appears in
// exts. Each value in exts is an optional transform applied to the raw source
// before parsing; it must preserve len(src) so that tree-sitter byte offsets
// remain valid. A nil transform means the source is used as-is.
func scanFiles(dir, fieldName string, exts map[string]func([]byte) []byte, lang *sitter.Language, walk walkFn, skipDirs map[string]bool) ([]Reference, int, error) {
	var refs []Reference
	filesScanned := 0

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if path != dir && (strings.HasPrefix(name, ".") || skipDirs[name]) {
				return filepath.SkipDir
			}
			return nil
		}
		transform, ok := exts[filepath.Ext(path)]
		if !ok {
			return nil
		}
		filesScanned++

		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		parseSrc := src
		if transform != nil {
			parseSrc = transform(src)
		}

		p := sitter.NewParser()
		p.SetLanguage(lang)
		tree, err := parseCtxFn(p, context.Background(), nil, parseSrc)
		if err != nil {
			return err
		}

		rel, err := filepathRelFn(dir, path)
		if err != nil {
			rel = filepath.Clean(path)
		}
		lines := bytes.Split(src, []byte("\n"))
		var fileRefs []Reference
		walk(tree.RootNode(), parseSrc, lines, fieldName, rel, &fileRefs)
		refs = append(refs, dedupeByLine(fileRefs)...)
		return nil
	})

	return refs, filesScanned, err
}

// dedupeByLine keeps the first Reference seen for each line number.
func dedupeByLine(refs []Reference) []Reference {
	seen := map[int]bool{}
	out := refs[:0:0]
	for _, r := range refs {
		if !seen[r.Line] {
			seen[r.Line] = true
			out = append(out, r)
		}
	}
	return out
}

func lineAt(lines [][]byte, row int) string {
	if row < len(lines) {
		return string(lines[row])
	}
	return ""
}

// walkNodeRuby matches Ruby method calls (call nodes) where the method name
// equals fieldName, e.g. user.email → matches "email".
// A receiver is required to avoid false positives on standalone method calls
// like raw(string) or send(msg) that share a name with the target field.
func walkNodeRuby(node *sitter.Node, src []byte, lines [][]byte, fieldName, file string, refs *[]Reference) {
	if node.Type() == "call" {
		method := node.ChildByFieldName("method")
		receiver := node.ChildByFieldName("receiver")
		if method != nil && receiver != nil && method.Content(src) == fieldName {
			methodRow := int(method.StartPoint().Row)
			text := node.Content(src)
			if int(node.StartPoint().Row) != methodRow {
				text = strings.TrimSpace(lineAt(lines, methodRow))
			}
			*refs = append(*refs, Reference{
				File: file,
				Line: methodRow + 1,
				Text: text,
			})
		}
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		walkNodeRuby(node.Child(i), src, lines, fieldName, file, refs)
	}
}

func erbToRuby(src []byte) []byte {
	out := make([]byte, len(src))
	inRuby := false
	inComment := false
	inString := byte(0) // '"' or '\'' when inside a Ruby string literal
	escaped := false
	i := 0
	for i < len(src) {
		if inComment {
			if i+1 < len(src) && src[i] == '%' && src[i+1] == '>' {
				out[i], out[i+1] = ' ', ' '
				i += 2
				inComment = false
			} else if src[i] == '\n' {
				out[i] = '\n'
				i++
			} else {
				out[i] = ' '
				i++
			}
		} else if inRuby {
			if inString != 0 {
				out[i] = src[i]
				if escaped {
					escaped = false
				} else if src[i] == '\\' {
					escaped = true
				} else if src[i] == inString {
					inString = 0
				}
				i++
			} else if i+1 < len(src) && src[i] == '%' && src[i+1] == '>' {
				// Terminate the ERB block as a Ruby statement so two
				// adjacent <%= a %> <%= b %> tags on the same source line
				// don't collapse into a single ambiguous call when the
				// surrounding HTML is converted to whitespace. Without the
				// `;`, tree-sitter parses the second tag as a continuation
				// of the first, dropping its `call` nodes (issue #64).
				out[i], out[i+1] = ';', ' '
				i += 2
				inRuby = false
			} else {
				if src[i] == '"' || src[i] == '\'' {
					inString = src[i]
				}
				out[i] = src[i]
				i++
			}
		} else {
			if i+1 < len(src) && src[i] == '<' && src[i+1] == '%' {
				out[i], out[i+1] = ' ', ' '
				i += 2
				if i < len(src) && src[i] == '#' {
					out[i] = ' '
					i++
					inComment = true
				} else {
					if i < len(src) && (src[i] == '=' || src[i] == '-') {
						out[i] = ' '
						i++
					}
					inRuby = true
				}
			} else if src[i] == '\n' {
				out[i] = '\n'
				i++
			} else {
				out[i] = ' '
				i++
			}
		}
	}
	return out
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
				case methodName == "F":
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

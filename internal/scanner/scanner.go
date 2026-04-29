package scanner

import (
	"bytes"
	"context"
	"io/fs"
	"os"
	"path/filepath"
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
	return Scan(dir, fieldName)
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

// parseCtxFn is the function used to parse Python source into a tree.
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
	return scanFiles(dir, fieldName, ".py", python.GetLanguage(), walkNode, nil, SkipDirs)
}

// ScanRuby walks dir and returns every method-call node whose method name
// equals fieldName, along with the total number of .rb and .erb files examined.
func ScanRuby(dir, fieldName string) ([]Reference, int, error) {
	refs, n1, err := scanFiles(dir, fieldName, ".rb", ruby.GetLanguage(), walkNodeRuby, nil, RubySkipDirs)
	if err != nil {
		return nil, n1, err
	}
	erbRefs, n2, err := scanFiles(dir, fieldName, ".erb", ruby.GetLanguage(), walkNodeRuby, erbToRuby, RubySkipDirs)
	if err != nil {
		return nil, n1 + n2, err
	}
	return append(refs, erbRefs...), n1 + n2, nil
}

type walkFn func(node *sitter.Node, src []byte, lines [][]byte, fieldName, file string, refs *[]Reference)

func scanFiles(dir, fieldName, ext string, lang *sitter.Language, walk walkFn, transform func([]byte) []byte, skipDirs map[string]bool) ([]Reference, int, error) {
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
		if !strings.HasSuffix(path, ext) {
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
func walkNodeRuby(node *sitter.Node, src []byte, lines [][]byte, fieldName, file string, refs *[]Reference) {
	if node.Type() == "call" {
		method := node.ChildByFieldName("method")
		if method != nil && method.Content(src) == fieldName {
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
			if i+1 < len(src) && src[i] == '%' && src[i+1] == '>' {
				out[i], out[i+1] = ' ', ' '
				i += 2
				inRuby = false
			} else {
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

package scanner

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/python"
)

type Reference struct {
	File string
	Line int
	Text string
}

// SkipDirs is the set of directory names that are never scanned.
// Exported so CLI code that walks for models.py can apply the same rules.
var SkipDirs = map[string]bool{
	"__pycache__":  true,
	"venv":         true,
	"migrations":   true,
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
	lang := python.GetLanguage()
	var refs []Reference
	filesScanned := 0

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if path != dir && (strings.HasPrefix(name, ".") || SkipDirs[name]) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".py") {
			return nil
		}
		filesScanned++

		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		p := sitter.NewParser()
		p.SetLanguage(lang)
		tree, err := parseCtxFn(p, context.Background(), nil, src)
		if err != nil {
			return err
		}

		rel, err := filepathRelFn(dir, path)
		if err != nil {
			rel = filepath.Clean(path)
		}
		var fileRefs []Reference
		walkNode(tree.RootNode(), src, fieldName, rel, &fileRefs)
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

func walkNode(node *sitter.Node, src []byte, fieldName, file string, refs *[]Reference) {
	if node.Type() == "attribute" {
		attr := node.ChildByFieldName("attribute")
		if attr != nil && attr.Content(src) == fieldName {
			*refs = append(*refs, Reference{
				File: file,
				Line: int(node.StartPoint().Row) + 1,
				Text: node.Content(src),
			})
			// The object subtree cannot itself end in the same attribute name
			// unless it is a coincidental deeper match, so keep recursing to
			// catch patterns like a.email.email (rare but valid).
		}
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		walkNode(node.Child(i), src, fieldName, file, refs)
	}
}

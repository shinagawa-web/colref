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

var skipDirs = map[string]bool{
	"__pycache__":  true,
	"venv":         true,
	"migrations":   true,
	"node_modules": true,
}

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
			if strings.HasPrefix(name, ".") || skipDirs[name] {
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
		tree, err := p.ParseCtx(context.Background(), nil, src)
		if err != nil {
			return err
		}

		rel, _ := filepath.Rel(dir, path)
		walkNode(tree.RootNode(), src, fieldName, rel, &refs)
		return nil
	})

	return refs, filesScanned, err
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

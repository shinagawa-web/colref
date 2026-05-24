package refs

import (
	"bytes"
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"

	"github.com/shinagawa-web/colref/internal/orm"
)

// Reference is a type alias for orm.Reference for backward compatibility.
type Reference = orm.Reference

// parseCtxFn is the function used to parse source into a tree.
// It is a var so tests can inject a failing version to cover error paths.
var parseCtxFn = func(p *sitter.Parser, ctx context.Context, oldTree *sitter.Tree, src []byte) (*sitter.Tree, error) {
	return p.ParseCtx(ctx, oldTree, src)
}

// filepathRelFn is the function used to compute relative paths.
// It is a var so tests can inject a failing version to cover error paths.
var filepathRelFn = filepath.Rel

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

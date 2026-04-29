package parser

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/ruby"
)

// ParseMigrations reads all .rb files under migrateDir in filename order
// (the timestamp prefix ensures chronological order), replays
// create_table / add_column / remove_column / rename_column / drop_table
// operations, and returns the resulting column definitions.
func ParseMigrations(migrateDir string) ([]Field, error) {
	entries, err := os.ReadDir(migrateDir)
	if err != nil {
		return nil, err
	}

	schema := map[string]map[string]bool{} // table → set of column names

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".rb") {
			continue
		}
		src, err := os.ReadFile(filepath.Join(migrateDir, e.Name()))
		if err != nil {
			return nil, err
		}
		if err := applyMigration(schema, src); err != nil {
			return nil, err
		}
	}

	var fields []Field
	for table, cols := range schema {
		model := tableToModel(table)
		for col := range cols {
			fields = append(fields, Field{Model: model, Name: col})
		}
	}
	return fields, nil
}

func applyMigration(schema map[string]map[string]bool, src []byte) error {
	p := sitter.NewParser()
	p.SetLanguage(ruby.GetLanguage())
	tree, err := parseCtxFnRuby(p, context.Background(), nil, src)
	if err != nil {
		return err
	}
	walkMigrationNode(tree.RootNode(), src, schema)
	return nil
}

func walkMigrationNode(node *sitter.Node, src []byte, schema map[string]map[string]bool) {
	if node.Type() == "call" {
		method := node.ChildByFieldName("method")
		receiver := node.ChildByFieldName("receiver")
		if receiver == nil && method != nil {
			switch method.Content(src) {
			case "create_table":
				migCreateTable(node, src, schema)
				return
			case "add_column":
				migAddColumn(node, src, schema)
				return
			case "remove_column":
				migRemoveColumn(node, src, schema)
				return
			case "rename_column":
				migRenameColumn(node, src, schema)
				return
			case "drop_table":
				migDropTable(node, src, schema)
				return
			}
		}
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		walkMigrationNode(node.Child(i), src, schema)
	}
}

func migCreateTable(callNode *sitter.Node, src []byte, schema map[string]map[string]bool) {
	args := callNode.ChildByFieldName("arguments")
	if args == nil {
		return
	}
	table := migArg(args, src, 0)
	if table == "" {
		return
	}
	if schema[table] == nil {
		schema[table] = map[string]bool{}
	}
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
		col := migArg(colArgs, src, 0)
		if col == "" {
			continue
		}
		schema[table][col] = true
	}
}

func migAddColumn(callNode *sitter.Node, src []byte, schema map[string]map[string]bool) {
	args := callNode.ChildByFieldName("arguments")
	if args == nil {
		return
	}
	table := migArg(args, src, 0)
	col := migArg(args, src, 1)
	if table == "" || col == "" {
		return
	}
	if schema[table] == nil {
		schema[table] = map[string]bool{}
	}
	schema[table][col] = true
}

func migRemoveColumn(callNode *sitter.Node, src []byte, schema map[string]map[string]bool) {
	args := callNode.ChildByFieldName("arguments")
	if args == nil {
		return
	}
	table := migArg(args, src, 0)
	col := migArg(args, src, 1)
	if table == "" || col == "" {
		return
	}
	if schema[table] != nil {
		delete(schema[table], col)
	}
}

func migRenameColumn(callNode *sitter.Node, src []byte, schema map[string]map[string]bool) {
	args := callNode.ChildByFieldName("arguments")
	if args == nil {
		return
	}
	table := migArg(args, src, 0)
	oldCol := migArg(args, src, 1)
	newCol := migArg(args, src, 2)
	if table == "" || oldCol == "" || newCol == "" {
		return
	}
	if schema[table] != nil {
		delete(schema[table], oldCol)
		schema[table][newCol] = true
	}
}

func migDropTable(callNode *sitter.Node, src []byte, schema map[string]map[string]bool) {
	args := callNode.ChildByFieldName("arguments")
	if args == nil {
		return
	}
	table := migArg(args, src, 0)
	if table == "" {
		return
	}
	delete(schema, table)
}

// migArg returns the nth string-or-symbol argument (0-indexed).
// Ruby strings ("col") and symbols (:col) are both accepted.
func migArg(argList *sitter.Node, src []byte, n int) string {
	count := 0
	for i := 0; i < int(argList.ChildCount()); i++ {
		val := migArgValue(argList.Child(i), src)
		if val != "" {
			if count == n {
				return val
			}
			count++
		}
	}
	return ""
}

func migArgValue(node *sitter.Node, src []byte) string {
	switch node.Type() {
	case "string":
		for j := 0; j < int(node.ChildCount()); j++ {
			c := node.Child(j)
			if c.Type() == "string_content" {
				return c.Content(src)
			}
		}
	case "simple_symbol":
		content := node.Content(src)
		if len(content) > 1 && content[0] == ':' {
			return content[1:]
		}
	}
	return ""
}

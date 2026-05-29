package refs

import (
	"sort"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/ruby"

	"github.com/shinagawa-web/colref/internal/orm"
)

// RubySkipDirs is the set of directory names skipped when scanning Ruby projects.
var RubySkipDirs = map[string]bool{
	"spec":         true,
	"test":         true,
	"vendor":       true,
	"migrate":      true,
	"node_modules": true,
}

// rubySymbolArgMethods are ActiveRecord methods that accept field names as
// positional symbol (or string) arguments.
var rubySymbolArgMethods = map[string]bool{
	"select": true, "order": true, "pluck": true,
	"pick": true, "group": true, "reorder": true,
	"update_column": true,
	"minimum":       true, "maximum": true, "sum": true,
	"average": true, "count": true,
}

// rubyHashKeyArgMethods are ActiveRecord methods that accept field names as
// hash keys. "not" is included but validated against a where receiver at call
// time to avoid matching unrelated DSL methods.
var rubyHashKeyArgMethods = map[string]bool{
	"where": true, "order": true, "reorder": true, "not": true,
	"new": true, "create": true, "find_or_create_by": true, "find_or_initialize_by": true,
	"update": true, "assign_attributes": true, "update_columns": true, "update_all": true,
	"find_by": true, "exists?": true,
}

// rubySqlMethods are Ruby/ActiveRecord methods whose first positional string
// argument is raw SQL.
var rubySqlMethods = map[string]bool{
	"find_by_sql": true, "execute": true, "select_all": true,
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

// ScanRuby combines attribute-access and string-based ORM scanning for Rails
// projects in a single parse pass per file. Results are sorted by (File, Line).
func ScanRuby(dir, fieldName string) ([]Reference, int, error) {
	combined := func(node *sitter.Node, src []byte, lines [][]byte, fn, file string, refs *[]Reference) {
		walkNodeRuby(node, src, lines, fn, file, refs)
		walkNodeRubyStringRefs(node, src, lines, fn, file, refs)
	}
	refs, count, err := scanFiles(dir, fieldName, map[string]func([]byte) []byte{
		".rb":  nil,
		".erb": erbToRuby,
	}, ruby.GetLanguage(), combined, RubySkipDirs)
	if err != nil {
		return nil, 0, err
	}
	sort.Slice(refs, func(i, j int) bool {
		if refs[i].File != refs[j].File {
			return refs[i].File < refs[j].File
		}
		return refs[i].Line < refs[j].Line
	})
	return refs, count, nil
}

// ScanRubyStringRefs walks dir and returns every string-based ActiveRecord
// reference to fieldName: symbol/hash-key args to known query methods, SQL
// string fragments in where("..."), and arel_table subscripts.
// Symbol and hash-key refs are prefixed "[string]"; SQL string fragments are
// prefixed "[sql ref]".
func ScanRubyStringRefs(dir, fieldName string) ([]Reference, int, error) {
	return scanFiles(dir, fieldName, map[string]func([]byte) []byte{
		".rb":  nil,
		".erb": erbToRuby,
	}, ruby.GetLanguage(), walkNodeRubyStringRefs, RubySkipDirs)
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

// walkNodeRubyStringRefs finds string-based ActiveRecord references to fieldName.
// It pre-collects the source rows of heredoc_beginning nodes inside rubySqlMethods
// calls so that the heredoc_body handler only fires for SQL method arguments.
func walkNodeRubyStringRefs(node *sitter.Node, src []byte, lines [][]byte, fieldName, file string, refs *[]Reference) {
	sqlHeredocRows := collectSqlHeredocRows(node, src)
	walkNodeRubyStringRefsInner(node, src, lines, fieldName, file, refs, sqlHeredocRows)
}

// collectSqlHeredocRows pre-scans the AST subtree rooted at root and returns
// the set of source rows on which heredoc_beginning nodes appear as positional
// arguments to rubySqlMethods calls. The heredoc_body sibling shares the same
// StartPoint().Row, so this set is used to constrain the heredoc_body handler.
func collectSqlHeredocRows(root *sitter.Node, src []byte) map[int]bool {
	rows := make(map[int]bool)
	var collect func(*sitter.Node)
	collect = func(node *sitter.Node) {
		if node.Type() == "call" {
			method := node.ChildByFieldName("method")
			args := node.ChildByFieldName("arguments")
			if method != nil && args != nil && rubySqlMethods[method.Content(src)] {
				for i := 0; i < int(args.ChildCount()); i++ {
					c := args.Child(i)
					if c.Type() == "heredoc_beginning" {
						rows[int(c.StartPoint().Row)] = true
					}
				}
			}
		}
		for i := 0; i < int(node.ChildCount()); i++ {
			collect(node.Child(i))
		}
	}
	collect(root)
	return rows
}

func walkNodeRubyStringRefsInner(node *sitter.Node, src []byte, lines [][]byte, fieldName, file string, refs *[]Reference, sqlHeredocRows map[int]bool) {
	switch node.Type() {
	case "call":
		method := node.ChildByFieldName("method")
		args := node.ChildByFieldName("arguments")
		if method == nil || args == nil {
			break
		}
		methodName := method.Content(src)

		if rubySqlMethods[methodName] {
			for i := 0; i < int(args.ChildCount()); i++ {
				child := args.Child(i)
				if child.Type() == "string" {
					content := rubyStringContent(child, src)
					if isSQLString(content) && sqlContainsField([]byte(content), fieldName) {
						addRubySqlRef(child, lines, file, refs)
					}
					break
				}
			}
			break
		}

		if methodName == "calculate" {
			// calculate(:operation, :column) — second positional symbol/string arg is the field.
			pos := 0
			for i := 0; i < int(args.ChildCount()); i++ {
				child := args.Child(i)
				if child.Type() == "simple_symbol" || child.Type() == "string" {
					if pos == 1 {
						if child.Type() == "simple_symbol" && rubySymbolName(child, src) == fieldName {
							addRubyStringRef(child, lines, file, refs)
						} else if child.Type() == "string" && rubyStringContent(child, src) == fieldName {
							addRubyStringRef(child, lines, file, refs)
						}
						break
					}
					pos++
				}
			}
			break
		}

		// For "not", only match when receiver is a call to "where".
		if methodName == "not" {
			receiver := node.ChildByFieldName("receiver")
			if receiver == nil || receiver.Type() != "call" {
				break
			}
			recvMethod := receiver.ChildByFieldName("method")
			if recvMethod == nil || recvMethod.Content(src) != "where" {
				break
			}
		}

		seenWhereString := false
		for i := 0; i < int(args.ChildCount()); i++ {
			child := args.Child(i)
			switch child.Type() {
			case "simple_symbol":
				if rubySymbolArgMethods[methodName] && rubySymbolName(child, src) == fieldName {
					addRubyStringRef(child, lines, file, refs)
				}
			case "pair":
				if rubyHashKeyArgMethods[methodName] {
					key := child.ChildByFieldName("key")
					if key != nil && key.Type() == "hash_key_symbol" && key.Content(src) == fieldName {
						addRubyStringRef(key, lines, file, refs)
					}
				}
			case "string":
				content := rubyStringContent(child, src)
				if rubySymbolArgMethods[methodName] {
					if content == fieldName {
						addRubyStringRef(child, lines, file, refs)
					} else if rubyContainsField(content, fieldName) {
						addRubySqlRef(child, lines, file, refs)
					}
				} else if methodName == "where" && !seenWhereString {
					seenWhereString = true
					if content == fieldName {
						addRubyStringRef(child, lines, file, refs)
					} else if rubyContainsField(content, fieldName) {
						addRubySqlRef(child, lines, file, refs)
					}
				}
			}
		}

	case "element_reference":
		// Article.arel_table[:field] or arel_table[:field] (implicit self)
		if node.ChildCount() >= 3 {
			receiver := node.Child(0)
			if receiver != nil {
				isArelTable := (receiver.Type() == "call" &&
					receiver.ChildByFieldName("method") != nil &&
					receiver.ChildByFieldName("method").Content(src) == "arel_table") ||
					(receiver.Type() == "identifier" && receiver.Content(src) == "arel_table")
				if isArelTable {
					for i := 1; i < int(node.ChildCount()); i++ {
						sym := node.Child(i)
						if sym.Type() == "simple_symbol" && rubySymbolName(sym, src) == fieldName {
							addRubyStringRef(node, lines, file, refs)
						}
					}
				}
			}
		}

	case "heredoc_body":
		// Only emit a ref when this heredoc was passed to a rubySqlMethod call.
		if !sqlHeredocRows[int(node.StartPoint().Row)] {
			break
		}
		var sb strings.Builder
		for i := 0; i < int(node.ChildCount()); i++ {
			c := node.Child(i)
			if c.Type() == "heredoc_content" {
				sb.WriteString(c.Content(src))
			}
		}
		content := sb.String()
		if isSQLString(content) && sqlContainsField([]byte(content), fieldName) {
			addRubySqlRef(node, lines, file, refs)
		}
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		walkNodeRubyStringRefsInner(node.Child(i), src, lines, fieldName, file, refs, sqlHeredocRows)
	}
}

// rubySymbolName returns the symbol name without the leading colon.
// Only called on simple_symbol nodes, which always have a colon prefix.
func rubySymbolName(node *sitter.Node, src []byte) string {
	return node.Content(src)[1:]
}

// rubyStringContent returns the concatenated text of all string_content children
// of a Ruby string node, skipping interpolations and other non-literal segments.
func rubyStringContent(node *sitter.Node, src []byte) string {
	var sb strings.Builder
	for i := 0; i < int(node.ChildCount()); i++ {
		c := node.Child(i)
		if c.Type() == "string_content" {
			sb.WriteString(c.Content(src))
		}
	}
	return sb.String()
}

// rubyContainsField reports whether s contains fieldName as a whole word
// (bounded by non-alphanumeric/non-underscore characters or string ends).
// All occurrences are checked so a non-boundary first hit does not mask a
// later boundary hit (e.g. "user_id id" correctly matches "id").
func rubyContainsField(s, fieldName string) bool {
	start := 0
	for {
		idx := strings.Index(s[start:], fieldName)
		if idx < 0 {
			return false
		}
		idx += start
		before := idx == 0 || !isWordChar(s[idx-1])
		after := idx+len(fieldName) == len(s) || !isWordChar(s[idx+len(fieldName)])
		if before && after {
			return true
		}
		start = idx + len(fieldName)
		if start >= len(s) {
			return false
		}
	}
}

func isWordChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_'
}

// addRubyStringRef appends a [string]-labeled Reference at the node's source row.
func addRubyStringRef(node *sitter.Node, lines [][]byte, file string, refs *[]Reference) {
	row := int(node.StartPoint().Row)
	*refs = append(*refs, Reference{
		File: file,
		Line: row + 1,
		Text: "[string] " + strings.TrimSpace(lineAt(lines, row)),
	})
}

// addRubySqlRef appends a [sql ref]-labeled Reference at the node's source row.
func addRubySqlRef(node *sitter.Node, lines [][]byte, file string, refs *[]Reference) {
	row := int(node.StartPoint().Row)
	*refs = append(*refs, Reference{
		File: file,
		Line: row + 1,
		Text: "[sql ref] " + strings.TrimSpace(lineAt(lines, row)),
	})
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

package schema

import (
	"bufio"
	"bytes"
	"strings"
)

// ParseStructureSql parses a Rails db/structure.sql file and returns all
// column definitions as Fields. It recognises CREATE TABLE blocks and
// ALTER TABLE … ADD COLUMN statements. The Model name is derived from the
// table name using the same singularize+CamelCase heuristic as ParseSchemaRb.
//
// Supported quoting styles: double-quoted ("col"), backtick-quoted (`col`),
// and unquoted. Optional schema prefixes (e.g. public.users) are stripped.
func ParseStructureSql(src []byte) ([]Field, error) {
	scanner := bufio.NewScanner(bytes.NewReader(src))
	var fields []Field

	inTable := false
	currentModel := ""

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		upper := strings.ToUpper(trimmed)

		if !inTable {
			switch {
			case strings.HasPrefix(upper, "CREATE TABLE"):
				if name := sqlExtractCreateTableName(trimmed); name != "" {
					currentModel = tableToModel(name)
					inTable = true
				}
			case strings.HasPrefix(upper, "ALTER TABLE"):
				table, col := sqlExtractAlterAddColumn(trimmed)
				if table != "" && col != "" {
					fields = append(fields, Field{Model: tableToModel(table), Name: col})
				}
			}
			continue
		}

		// Inside a CREATE TABLE block.
		if strings.HasPrefix(trimmed, ")") {
			inTable = false
			currentModel = ""
			continue
		}
		if sqlIsConstraintLine(upper) {
			continue
		}
		if col := sqlFirstIdent(trimmed); col != "" {
			fields = append(fields, Field{Model: currentModel, Name: col})
		}
	}

	return fields, scanner.Err()
}

// sqlExtractCreateTableName extracts the bare table name from a
// "CREATE TABLE [IF NOT EXISTS] [schema.]name (" line.
func sqlExtractCreateTableName(line string) string {
	upper := strings.ToUpper(line)
	idx := strings.Index(upper, "CREATE TABLE")
	if idx < 0 {
		return ""
	}
	rest := strings.TrimSpace(line[idx+len("CREATE TABLE"):])

	// Strip optional IF NOT EXISTS.
	if up := strings.ToUpper(rest); strings.HasPrefix(up, "IF NOT EXISTS") {
		rest = strings.TrimSpace(rest[len("IF NOT EXISTS"):])
	}

	// First identifier may be the schema prefix.
	ident1, rest2 := sqlNextIdent(rest)
	if ident1 == "" {
		return ""
	}

	// If followed by '.', the real table name is the next identifier.
	if len(strings.TrimSpace(rest2)) > 0 && strings.TrimSpace(rest2)[0] == '.' {
		rest2 = strings.TrimSpace(rest2)[1:]
		if ident2, _ := sqlNextIdent(rest2); ident2 != "" {
			return ident2
		}
	}

	return ident1
}

// sqlExtractAlterAddColumn extracts the table name and column name from an
// "ALTER TABLE [ONLY] [schema.]table ADD [COLUMN] [IF NOT EXISTS] col …" line.
// Returns ("", "") if the line is not a matching ADD COLUMN statement.
func sqlExtractAlterAddColumn(line string) (table, col string) {
	upper := strings.ToUpper(line)
	idx := strings.Index(upper, "ALTER TABLE")
	if idx < 0 {
		return "", ""
	}
	rest := strings.TrimSpace(line[idx+len("ALTER TABLE"):])

	// Strip optional ONLY.
	if up := strings.ToUpper(rest); strings.HasPrefix(up, "ONLY") &&
		(len(up) == 4 || !sqlIsWordByte(up[4])) {
		rest = strings.TrimSpace(rest[4:])
	}

	// Extract table name (handle schema.table).
	ident1, rest2 := sqlNextIdent(rest)
	if ident1 == "" {
		return "", ""
	}
	rest2 = strings.TrimSpace(rest2)
	if len(rest2) > 0 && rest2[0] == '.' {
		if ident2, r := sqlNextIdent(strings.TrimSpace(rest2[1:])); ident2 != "" {
			table, rest2 = ident2, r
		} else {
			table = ident1
		}
	} else {
		table = ident1
	}

	// Find the ADD keyword. Trim rest2 first so addIdx indexes into it correctly.
	rest2 = strings.TrimSpace(rest2)
	up2 := strings.ToUpper(rest2)
	addIdx := strings.Index(up2, "ADD")
	if addIdx < 0 || (addIdx+3 < len(up2) && sqlIsWordByte(up2[addIdx+3])) {
		return "", ""
	}
	rest3 := strings.TrimSpace(rest2[addIdx+3:])

	// Strip optional COLUMN.
	if up3 := strings.ToUpper(rest3); strings.HasPrefix(up3, "COLUMN") &&
		(len(up3) == 6 || !sqlIsWordByte(up3[6])) {
		rest3 = strings.TrimSpace(rest3[6:])
	}

	// Strip optional IF NOT EXISTS.
	if up3 := strings.ToUpper(rest3); strings.HasPrefix(up3, "IF NOT EXISTS") {
		rest3 = strings.TrimSpace(rest3[len("IF NOT EXISTS"):])
	}

	// Skip non-column ADD clauses: ADD CONSTRAINT, ADD PRIMARY KEY, etc.
	up3 := strings.ToUpper(rest3)
	for _, kw := range []string{"CONSTRAINT", "PRIMARY", "UNIQUE", "FOREIGN", "CHECK", "INDEX"} {
		if strings.HasPrefix(up3, kw) && (len(up3) == len(kw) || !sqlIsWordByte(up3[len(kw)])) {
			return "", ""
		}
	}

	col, _ = sqlNextIdent(rest3)
	return table, col
}

// sqlNextIdent returns the first SQL identifier from s (double-quoted, backtick-
// quoted, or a plain word) and the remaining string after it.
func sqlNextIdent(s string) (ident, rest string) {
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return "", ""
	}
	switch s[0] {
	case '"':
		end := strings.IndexByte(s[1:], '"')
		if end < 0 {
			return "", ""
		}
		return s[1 : end+1], s[end+2:]
	case '`':
		end := strings.IndexByte(s[1:], '`')
		if end < 0 {
			return "", ""
		}
		return s[1 : end+1], s[end+2:]
	default:
		end := strings.IndexAny(s, " \t\n\r(),;.")
		if end < 0 {
			return s, ""
		}
		return s[:end], s[end:]
	}
}

// sqlFirstIdent returns the first identifier from a column-definition line.
// It is the column name for lines inside a CREATE TABLE block.
func sqlFirstIdent(line string) string {
	name, _ := sqlNextIdent(strings.TrimSpace(line))
	return name
}

// sqlConstraintPrefixes are keywords that start a table constraint line
// inside a CREATE TABLE block (not a column definition).
var sqlConstraintPrefixes = []string{
	"PRIMARY", "UNIQUE", "KEY", "CONSTRAINT", "FOREIGN", "INDEX", "CHECK", "EXCLUDE",
}

// sqlIsConstraintLine reports whether an upper-cased, trimmed line is a
// table constraint rather than a column definition.
func sqlIsConstraintLine(upperTrimmed string) bool {
	for _, kw := range sqlConstraintPrefixes {
		if strings.HasPrefix(upperTrimmed, kw) {
			return true
		}
	}
	return false
}

// sqlIsWordByte reports whether b is an ASCII identifier character.
func sqlIsWordByte(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9') || b == '_'
}

package orm

type Field struct {
	Model string
	Name  string
}

type Reference struct {
	File string
	Line int
	Text string
}

type SchemaParser interface {
	ParseSchema(src []byte) ([]Field, error)
}

type ReferenceScanner interface {
	Scan(dir, fieldName string) ([]Reference, int, error)
	// SkipDirs returns a copy of the directory names that are never scanned.
	SkipDirs() map[string]bool
}

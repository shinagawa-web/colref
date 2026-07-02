package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/shinagawa-web/colref/internal/orm"
	"github.com/shinagawa-web/colref/internal/refs"
	"github.com/shinagawa-web/colref/internal/schema"
	"github.com/spf13/cobra"
)

var (
	flagModel  string
	flagField  string
	flagOrm    string
	flagFormat string
)

var checkCmd = &cobra.Command{
	Use:   "check [path]",
	Short: "Scan a codebase for references to a model field",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := "."
		if len(args) == 1 {
			dir = args[0]
		}
		if err := validateFormat(flagFormat); err != nil {
			return err
		}
		return runCheck(dir, flagModel, flagField, flagOrm)
	},
}

func init() {
	checkCmd.Flags().StringVar(&flagModel, "model", "", "Model name (e.g. User)")
	checkCmd.Flags().StringVar(&flagField, "field", "", "Field name (e.g. email)")
	checkCmd.Flags().StringVar(&flagOrm, "orm", "", "ORM type: django, rails")
	checkCmd.Flags().StringVar(&flagFormat, "format", "text", "Output format: text, json")
	_ = checkCmd.MarkFlagRequired("model")
	_ = checkCmd.MarkFlagRequired("field")
	_ = checkCmd.MarkFlagRequired("orm")

	rootCmd.AddCommand(checkCmd)
}

// buildModelSet is the function used to build the cross-file Django model set.
// It is a var so tests can inject a failing version to cover error paths.
var buildModelSet = schema.BuildModelSet

// parseModelsWithSet is the function used to extract fields from a source file
// given a pre-built model set. It is a var so tests can inject a failing version.
var parseModelsWithSet = schema.ParseModelsWithSet

// parseSchemaRb is the function used to parse a db/schema.rb source file.
// It is a var so tests can inject a failing version to cover error paths.
var parseSchemaRb = schema.ParseSchemaRb

// parseStructureSql is the function used to parse a db/structure.sql source file.
// It is a var so tests can inject a failing version to cover error paths.
var parseStructureSql = schema.ParseStructureSql

// parseMigrations is the function used to parse db/migrate/ files.
// It is a var so tests can inject a failing version to cover error paths.
var parseMigrations = schema.ParseMigrations

// filepathRelFn is the function used to compute relative paths in runCheck.
// It is a var so tests can inject a failing version to cover error paths.
var filepathRelFn = filepath.Rel

func runCheck(dir, modelName, fieldName, ormName string) error {
	switch ormName {
	case "django":
		return runCheckDjango(dir, modelName, fieldName)
	case "rails":
		return runCheckRails(dir, modelName, fieldName)
	default:
		return fmt.Errorf("unknown --orm %q: supported values are django, rails", ormName)
	}
}

func runCheckRails(dir, modelName, fieldName string) error {
	// 1. db/schema.rb (preferred).
	schemaFile := filepath.Join(dir, "db", "schema.rb")
	src, err := os.ReadFile(schemaFile)
	if err == nil {
		fields, err := parseSchemaRb(src)
		if err != nil {
			return fmt.Errorf("parse %s: %w", schemaFile, err)
		}
		return runCheckFields(dir, modelName, fieldName, "rails", fields, refs.ScanRuby)
	}
	if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read %s: %w", schemaFile, err)
	}

	// 2. db/structure.sql (used when config.active_record.schema_format = :sql).
	structureFile := filepath.Join(dir, "db", "structure.sql")
	src, err = os.ReadFile(structureFile)
	if err == nil {
		fields, err := parseStructureSql(src)
		if err != nil {
			return fmt.Errorf("parse %s: %w", structureFile, err)
		}
		return runCheckFields(dir, modelName, fieldName, "rails", fields, refs.ScanRuby)
	}
	if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read %s: %w", structureFile, err)
	}

	// 3. db/migrate/ (fallback when no schema dump is present).
	return runCheckRailsMigrations(dir, modelName, fieldName)
}

func runCheckRailsMigrations(dir, modelName, fieldName string) error {
	migrateDir := filepath.Join(dir, "db", "migrate")
	fields, err := parseMigrations(migrateDir)
	if err != nil {
		return fmt.Errorf("parse migrations from %s: %w", migrateDir, err)
	}
	return runCheckFields(dir, modelName, fieldName, "rails", fields, refs.ScanRuby)
}

func runCheckDjango(dir, modelName, fieldName string) error {
	modelsFiles, err := findModelsFiles(dir)
	if err != nil {
		return err
	}
	if len(modelsFiles) == 0 {
		return fmt.Errorf("no models.py, abstract_models.py, or models/*.py found under %s", dir)
	}

	// Read all sources first so we can build a cross-file model set.
	sources := make([][]byte, len(modelsFiles))
	for i, f := range modelsFiles {
		src, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		sources[i] = src
	}

	// Phase 1: build the Django model set using transitive closure across all files.
	modelSet, err := buildModelSet(sources)
	if err != nil {
		return fmt.Errorf("build model set: %w", err)
	}

	// Phase 2: extract fields per file using the cross-file model set.
	type parsedFile struct {
		path   string
		fields []orm.Field
	}
	var parsed []parsedFile
	for i, f := range modelsFiles {
		fields, err := parseModelsWithSet(sources[i], modelSet)
		if err != nil {
			return fmt.Errorf("parse %s: %w", f, err)
		}
		parsed = append(parsed, parsedFile{path: f, fields: fields})
	}

	modelToFiles := map[string][]string{}
	for _, pf := range parsed {
		seen := map[string]bool{}
		for _, field := range pf.fields {
			if !seen[field.Model] {
				seen[field.Model] = true
				modelToFiles[field.Model] = append(modelToFiles[field.Model], pf.path)
			}
		}
	}

	if files := modelToFiles[modelName]; len(files) > 1 {
		lines := []string{fmt.Sprintf("model %q found in multiple files:", modelName)}
		for _, f := range files {
			rel, err := filepathRelFn(dir, f)
			if err != nil {
				rel = filepath.Clean(f)
			}
			lines = append(lines, "  "+rel)
		}
		lines = append(lines, "Use --model to disambiguate.")
		return fmt.Errorf("%s", strings.Join(lines, "\n"))
	}

	var allFields []orm.Field
	for _, pf := range parsed {
		allFields = append(allFields, pf.fields...)
	}

	return runCheckFields(dir, modelName, fieldName, "django", allFields, refs.ScanDjango)
}

func runCheckFields(dir, modelName, fieldName, ormName string, allFields []schema.Field, scan func(string, string) ([]refs.Reference, int, error)) error {
	fieldNames := fieldsForModel(allFields, modelName)
	if len(fieldNames) == 0 {
		known := knownModels(allFields)
		if len(known) == 0 {
			return fmt.Errorf("model %q not found (no models detected)", modelName)
		}
		return fmt.Errorf("model %q not found\nAvailable models: %s", modelName, strings.Join(known, ", "))
	}

	if !containsField(fieldNames, fieldName) {
		return fmt.Errorf("field %q not found in model %q\nAvailable fields: %s",
			fieldName, modelName, strings.Join(fieldNames, ", "))
	}

	return runScan(dir, modelName, fieldName, ormName, scan)
}

func runScan(dir, modelName, fieldName, ormName string, scan func(string, string) ([]refs.Reference, int, error)) error {
	references, count, err := scan(dir, fieldName)
	if err != nil {
		return err
	}

	if flagFormat == "json" {
		result := buildResult(modelName, fieldName, ormName, count, references)
		return printJSON(os.Stdout, result)
	}

	fmt.Printf("Scanning %d files...\n\n", count)

	if len(references) == 0 {
		fmt.Printf("No references found for %s.%s\n\n", modelName, fieldName)
		fmt.Printf("  Verify manually before deleting.\n")
		return nil
	}

	fmt.Printf("References found for %s.%s\n\n", modelName, fieldName)
	printRefs(references)
	return nil
}

// checkResult is the machine-readable form of a scan, emitted with --format json.
type checkResult struct {
	Model          string        `json:"model"`
	Field          string        `json:"field"`
	Orm            string        `json:"orm"`
	FilesScanned   int           `json:"files_scanned"`
	ReferenceCount int           `json:"reference_count"`
	References     []checkRefOut `json:"references"`
}

type checkRefOut struct {
	File string `json:"file"`
	Line int    `json:"line"`
	Text string `json:"text"`
}

// buildResult assembles the JSON result. It is pure (no I/O) so tests can assert
// on the structure directly without capturing stdout.
func buildResult(modelName, fieldName, ormName string, filesScanned int, references []orm.Reference) checkResult {
	out := make([]checkRefOut, 0, len(references))
	for _, r := range references {
		out = append(out, checkRefOut{File: r.File, Line: r.Line, Text: r.Text})
	}
	return checkResult{
		Model:          modelName,
		Field:          fieldName,
		Orm:            ormName,
		FilesScanned:   filesScanned,
		ReferenceCount: len(out),
		References:     out,
	}
}

func printJSON(w io.Writer, result checkResult) error {
	enc := json.NewEncoder(w)
	// The Text field holds source snippets (Ruby lambdas `->`, safe-nav `&.`, ERB
	// `<%= %>`). Disable HTML escaping so those stay readable on the wire for tools
	// consuming the JSON, rather than as the Unicode escapes `\u003c` / `\u003e` / `\u0026`
	// that encoding/json emits for <, >, and & by default.
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}

func validateFormat(format string) error {
	switch format {
	case "text", "json":
		return nil
	default:
		return fmt.Errorf("unknown --format %q: supported values are text, json", format)
	}
}

func printRefs(refs []orm.Reference) {
	maxWidth := 0
	for _, r := range refs {
		if w := len(fmt.Sprintf("%s:%d", r.File, r.Line)); w > maxWidth {
			maxWidth = w
		}
	}
	for _, r := range refs {
		label := fmt.Sprintf("%s:%d", r.File, r.Line)
		fmt.Printf("  %s%s%s\n", label, strings.Repeat(" ", maxWidth-len(label)+3), r.Text)
	}
}

func findModelsFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if path != dir && (strings.HasPrefix(name, ".") || refs.SkipDirs[name]) {
				return filepath.SkipDir
			}
			return nil
		}
		base := filepath.Base(path)
		if base == "models.py" || base == "abstract_models.py" || (filepath.Ext(path) == ".py" && filepath.Base(filepath.Dir(path)) == "models") {
			files = append(files, path)
		}
		return nil
	})
	sort.Strings(files)
	return files, err
}

func fieldsForModel(fields []orm.Field, model string) []string {
	var names []string
	for _, f := range fields {
		if f.Model == model {
			names = append(names, f.Name)
		}
	}
	return names
}

func knownModels(fields []orm.Field) []string {
	seen := map[string]bool{}
	var models []string
	for _, f := range fields {
		if !seen[f.Model] {
			seen[f.Model] = true
			models = append(models, f.Model)
		}
	}
	return models
}

func containsField(fields []string, name string) bool {
	for _, f := range fields {
		if f == name {
			return true
		}
	}
	return false
}

package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/shinagawa-web/colref/internal/orm"
	"github.com/shinagawa-web/colref/internal/parser"
	"github.com/shinagawa-web/colref/internal/scanner"
)

// parseModels is the function used to parse a models.py source file.
// It is a var so tests can inject a failing version to cover error paths.
var parseModels = parser.ParseModels

// parseSchemaRb is the function used to parse a db/schema.rb source file.
// It is a var so tests can inject a failing version to cover error paths.
var parseSchemaRb = parser.ParseSchemaRb

// filepathRelFn is the function used to compute relative paths in runCheck.
// It is a var so tests can inject a failing version to cover error paths.
var filepathRelFn = filepath.Rel

func runCheck(dir, modelName, fieldName, modelsFile, schemaFile string) error {
	// Auto-detect Rails when db/schema.rb exists (or --schema-file is set).
	if schemaFile == "" {
		candidate := filepath.Join(dir, "db", "schema.rb")
		if _, err := os.Stat(candidate); err == nil {
			schemaFile = candidate
		}
	}
	if schemaFile != "" {
		return runCheckRails(dir, modelName, fieldName, schemaFile)
	}
	return runCheckDjango(dir, modelName, fieldName, modelsFile)
}

func runCheckRails(dir, modelName, fieldName, schemaFile string) error {
	src, err := os.ReadFile(schemaFile)
	if err != nil {
		return err
	}
	fields, err := parseSchemaRb(src)
	if err != nil {
		return fmt.Errorf("parse %s: %w", schemaFile, err)
	}
	return runCheckFields(dir, modelName, fieldName, fields, scanner.ScanRuby, "Use --schema-file to specify a different schema.")
}

func runCheckDjango(dir, modelName, fieldName, modelsFile string) error {
	var modelsFiles []string
	if modelsFile != "" {
		modelsFiles = []string{modelsFile}
	} else {
		var err error
		modelsFiles, err = findModelsFiles(dir)
		if err != nil {
			return err
		}
		if len(modelsFiles) == 0 {
			return fmt.Errorf("no models.py found under %s", dir)
		}
	}

	type parsedFile struct {
		path   string
		fields []orm.Field
	}
	var parsed []parsedFile
	for _, f := range modelsFiles {
		src, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		fields, err := parseModels(src)
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
		lines = append(lines, "Use --models-file to specify which one.")
		return fmt.Errorf("%s", strings.Join(lines, "\n"))
	}

	var allFields []orm.Field
	for _, pf := range parsed {
		allFields = append(allFields, pf.fields...)
	}

	return runCheckFields(dir, modelName, fieldName, allFields, scanner.Scan, "Use --models-file to specify which one.")
}

func runCheckFields(dir, modelName, fieldName string, allFields []parser.Field, scan func(string, string) ([]scanner.Reference, int, error), _ string) error {
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

	refs, count, err := scan(dir, fieldName)
	if err != nil {
		return err
	}

	fmt.Printf("Scanning %d files...\n\n", count)

	if len(refs) == 0 {
		fmt.Printf("No references found for %s.%s\n\n", modelName, fieldName)
		fmt.Printf("  String-based ORM calls (e.g. .values(), .defer()) are not detected.\n")
		fmt.Printf("  Verify manually before deleting.\n")
		return nil
	}

	fmt.Printf("References found for %s.%s\n\n", modelName, fieldName)
	printRefs(refs)
	return nil
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
			if path != dir && (strings.HasPrefix(name, ".") || scanner.SkipDirs[name]) {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Base(path) == "models.py" {
			files = append(files, path)
		}
		return nil
	})
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

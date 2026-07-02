package e2e

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "overwrite golden files with current output")

const binaryName = "colref-e2e-test"

func run(t *testing.T, args ...string) ([]byte, error) {
	t.Helper()
	if _, err := os.Stat("./" + binaryName); err != nil {
		t.Fatalf("e2e binary not found: %v — run 'make build-e2e' first", err)
	}
	cmd := exec.Command("./"+binaryName, args...)
	out, err := cmd.CombinedOutput()
	return out, err
}

func assertContains(t *testing.T, out []byte, want string) {
	t.Helper()
	if !bytes.Contains(out, []byte(want)) {
		t.Errorf("expected output to contain %q\ngot:\n%s", want, out)
	}
}

func assertNotContains(t *testing.T, out []byte, unwanted string) {
	t.Helper()
	if bytes.Contains(out, []byte(unwanted)) {
		t.Errorf("expected output NOT to contain %q\ngot:\n%s", unwanted, out)
	}
}

func TestE2E_Django(t *testing.T) {
	fixture := "fixtures/django"

	t.Run("RefsFound", func(t *testing.T) {
		out, err := run(t, "check", "--orm", "django", "--model", "User", "--field", "email", fixture)
		if err != nil {
			t.Fatalf("unexpected error: %v\noutput:\n%s", err, out)
		}
		assertContains(t, out, "References found for User.email")
		assertContains(t, out, "accounts/views.py")
		assertNotContains(t, out, "No references found")
	})

	t.Run("NoRefs", func(t *testing.T) {
		out, err := run(t, "check", "--orm", "django", "--model", "User", "--field", "name", fixture)
		if err != nil {
			t.Fatalf("unexpected error: %v\noutput:\n%s", err, out)
		}
		assertContains(t, out, "No references found for User.name")
		assertContains(t, out, "Verify manually before deleting.")
		assertNotContains(t, out, "References found for")
	})

	t.Run("UnknownModel", func(t *testing.T) {
		out, err := run(t, "check", "--orm", "django", "--model", "Invoice", "--field", "amount", fixture)
		if err == nil {
			t.Fatal("expected non-zero exit for unknown model")
		}
		assertContains(t, out, `"Invoice" not found`)
		assertContains(t, out, "Available models:")
	})

	t.Run("UnknownField", func(t *testing.T) {
		out, err := run(t, "check", "--orm", "django", "--model", "User", "--field", "phone", fixture)
		if err == nil {
			t.Fatal("expected non-zero exit for unknown field")
		}
		assertContains(t, out, `"phone" not found`)
		assertContains(t, out, "Available fields:")
	})

	t.Run("CrossFileInheritance", func(t *testing.T) {
		// Order inherits from TrackedItem (defined in base/models.py), which in turn
		// inherits from models.Model. This verifies that Order is recognized as a
		// valid Django model via cross-file transitive closure.
		out, err := run(t, "check", "--orm", "django", "--model", "Order", "--field", "total", fixture)
		if err != nil {
			t.Fatalf("model Order should be detected via cross-file inheritance: %v\noutput:\n%s", err, out)
		}
		// No view files reference Order.total, so "No references found" is expected.
		assertContains(t, out, "No references found for Order.total")
	})
}

func TestE2E_Rails(t *testing.T) {
	fixture := "fixtures/rails"

	t.Run("RefsFound", func(t *testing.T) {
		out, err := run(t, "check", "--orm", "rails", "--model", "User", "--field", "email", fixture)
		if err != nil {
			t.Fatalf("unexpected error: %v\noutput:\n%s", err, out)
		}
		assertContains(t, out, "References found for User.email")
		assertContains(t, out, "app/user.rb")
		assertContains(t, out, "app/views/user.html.erb")
	})

	t.Run("NoRefs", func(t *testing.T) {
		out, err := run(t, "check", "--orm", "rails", "--model", "User", "--field", "name", fixture)
		if err != nil {
			t.Fatalf("unexpected error: %v\noutput:\n%s", err, out)
		}
		assertContains(t, out, "No references found for User.name")
		assertContains(t, out, "Verify manually before deleting.")
	})

	t.Run("UnknownModel", func(t *testing.T) {
		out, err := run(t, "check", "--orm", "rails", "--model", "Invoice", "--field", "amount", fixture)
		if err == nil {
			t.Fatal("expected non-zero exit for unknown model")
		}
		assertContains(t, out, `"Invoice" not found`)
	})

	t.Run("UnknownField", func(t *testing.T) {
		out, err := run(t, "check", "--orm", "rails", "--model", "User", "--field", "phone", fixture)
		if err == nil {
			t.Fatal("expected non-zero exit for unknown field")
		}
		assertContains(t, out, `"phone" not found`)
	})

	t.Run("NoSchemaFile", func(t *testing.T) {
		// schema.rb absent: fall back to db/migrate/ for field validation.
		fixture := "fixtures/rails-no-schema"
		out, err := run(t, "check", "--orm", "rails", "--model", "User", "--field", "email", fixture)
		if err != nil {
			t.Fatalf("no-schema path should not error: %v\noutput:\n%s", err, out)
		}
		assertContains(t, out, "References found for User.email")
		assertContains(t, out, "app/user.rb")
	})
}

func TestE2E_JSONOutput(t *testing.T) {
	fixture := "fixtures/django"

	t.Run("RefsFound", func(t *testing.T) {
		out, err := run(t, "check", "--orm", "django", "--model", "User", "--field", "email", "--format", "json", fixture)
		if err != nil {
			t.Fatalf("unexpected error: %v\noutput:\n%s", err, out)
		}
		// stdout must be pure, parseable JSON — no "Scanning..." preamble.
		assertNotContains(t, out, "Scanning")
		assertNotContains(t, out, "References found")

		var result struct {
			Model          string `json:"model"`
			Field          string `json:"field"`
			Orm            string `json:"orm"`
			FilesScanned   int    `json:"files_scanned"`
			ReferenceCount int    `json:"reference_count"`
			References     []struct {
				File string `json:"file"`
				Line int    `json:"line"`
				Text string `json:"text"`
			} `json:"references"`
		}
		if err := json.Unmarshal(out, &result); err != nil {
			t.Fatalf("output is not valid JSON: %v\noutput:\n%s", err, out)
		}
		if result.Model != "User" || result.Field != "email" || result.Orm != "django" {
			t.Errorf("unexpected identity fields: %+v", result)
		}
		if result.ReferenceCount == 0 || result.ReferenceCount != len(result.References) {
			t.Errorf("ReferenceCount = %d, len(References) = %d", result.ReferenceCount, len(result.References))
		}
	})

	t.Run("NoRefs", func(t *testing.T) {
		out, err := run(t, "check", "--orm", "django", "--model", "User", "--field", "name", "--format", "json", fixture)
		if err != nil {
			t.Fatalf("unexpected error: %v\noutput:\n%s", err, out)
		}
		var result struct {
			ReferenceCount int               `json:"reference_count"`
			References     []json.RawMessage `json:"references"`
		}
		if err := json.Unmarshal(out, &result); err != nil {
			t.Fatalf("output is not valid JSON: %v\noutput:\n%s", err, out)
		}
		if result.ReferenceCount != 0 {
			t.Errorf("ReferenceCount = %d, want 0", result.ReferenceCount)
		}
		// references must be [] (present), not null/omitted.
		assertContains(t, out, `"references": []`)
	})

	t.Run("InvalidFormat", func(t *testing.T) {
		out, err := run(t, "check", "--orm", "django", "--model", "User", "--field", "email", "--format", "xml", fixture)
		if err == nil {
			t.Fatal("expected non-zero exit for unknown format")
		}
		assertContains(t, out, `unknown --format "xml"`)
	})

	t.Run("Rails", func(t *testing.T) {
		// Exercise the Rails scan path (schema.rb + ERB views) through JSON output.
		out, err := run(t, "check", "--orm", "rails", "--model", "User", "--field", "email", "--format", "json", "fixtures/rails")
		if err != nil {
			t.Fatalf("unexpected error: %v\noutput:\n%s", err, out)
		}
		assertNotContains(t, out, "Scanning")

		var result struct {
			Orm            string `json:"orm"`
			ReferenceCount int    `json:"reference_count"`
			References     []struct {
				File string `json:"file"`
			} `json:"references"`
		}
		if err := json.Unmarshal(out, &result); err != nil {
			t.Fatalf("output is not valid JSON: %v\noutput:\n%s", err, out)
		}
		if result.Orm != "rails" {
			t.Errorf("orm = %q, want \"rails\"", result.Orm)
		}
		if result.ReferenceCount != len(result.References) {
			t.Errorf("ReferenceCount = %d, len(References) = %d", result.ReferenceCount, len(result.References))
		}
		// The .erb view references email and must be scanned alongside the .rb file.
		var sawERB bool
		for _, r := range result.References {
			if strings.HasSuffix(r.File, ".erb") {
				sawERB = true
			}
		}
		if !sawERB {
			t.Errorf("expected an .erb reference in JSON output\ngot:\n%s", out)
		}
	})

	t.Run("HTMLNotEscaped", func(t *testing.T) {
		// The Text field carries source snippets; HTML escaping is disabled so Ruby
		// lambdas (->), safe navigation (&.), and ERB (<%= %>) stay readable rather
		// than being emitted as > / &. The rails pattern battery contains
		// snippets with exactly these characters.
		out, err := run(t, "check", "--orm", "rails", "--model", "Article", "--field", "title", "--format", "json", "../test_patterns/rails")
		if err != nil {
			t.Fatalf("unexpected error: %v\noutput:\n%s", err, out)
		}
		// Must still be valid, parseable JSON.
		var result struct {
			References []json.RawMessage `json:"references"`
		}
		if err := json.Unmarshal(out, &result); err != nil {
			t.Fatalf("output is not valid JSON: %v\noutput:\n%s", err, out)
		}
		// Raw characters present, Unicode escapes absent.
		assertContains(t, out, "->(t)")
		assertContains(t, out, "article&.title")
		assertNotContains(t, out, `\u003e`) // escaped '>'
		assertNotContains(t, out, `\u0026`) // escaped '&'
		assertNotContains(t, out, `\u003c`) // escaped '<'
	})
}

func TestE2E_Django_ModelsPackage(t *testing.T) {
	fixture := "fixtures/django-models-pkg"

	t.Run("RefsFound", func(t *testing.T) {
		out, err := run(t, "check", "--orm", "django", "--model", "User", "--field", "email", fixture)
		if err != nil {
			t.Fatalf("unexpected error: %v\noutput:\n%s", err, out)
		}
		assertContains(t, out, "References found for User.email")
		assertContains(t, out, "accounts/views.py")
	})

	t.Run("NoRefs", func(t *testing.T) {
		out, err := run(t, "check", "--orm", "django", "--model", "User", "--field", "name", fixture)
		if err != nil {
			t.Fatalf("unexpected error: %v\noutput:\n%s", err, out)
		}
		assertContains(t, out, "No references found for User.name")
	})
}

func TestE2E_Django_AbstractModels(t *testing.T) {
	fixture := "fixtures/django-abstract-models"

	t.Run("RefsFound", func(t *testing.T) {
		out, err := run(t, "check", "--orm", "django", "--model", "AbstractProductClass", "--field", "name", fixture)
		if err != nil {
			t.Fatalf("unexpected error: %v\noutput:\n%s", err, out)
		}
		assertContains(t, out, "References found for AbstractProductClass.name")
		assertContains(t, out, "catalogue/views.py")
	})

	t.Run("NoRefs", func(t *testing.T) {
		out, err := run(t, "check", "--orm", "django", "--model", "AbstractProductClass", "--field", "slug", fixture)
		if err != nil {
			t.Fatalf("unexpected error: %v\noutput:\n%s", err, out)
		}
		assertContains(t, out, "No references found for AbstractProductClass.slug")
	})
}

func TestE2E_Django_Conflict(t *testing.T) {
	out, err := run(t, "check", "--orm", "django", "--model", "User", "--field", "email", "fixtures/django-conflict")
	if err == nil {
		t.Fatal("expected non-zero exit for duplicate model definition")
	}
	assertContains(t, out, "multiple files")
	assertContains(t, out, "app1/models.py")
	assertContains(t, out, "app2/models.py")
}

func TestE2E_Rails_Migrations(t *testing.T) {
	fixture := "fixtures/rails-migrations"

	t.Run("AddColumnRefsFound", func(t *testing.T) {
		// age is added via add_column, not present in create_table.
		out, err := run(t, "check", "--orm", "rails", "--model", "User", "--field", "age", fixture)
		if err != nil {
			t.Fatalf("unexpected error: %v\noutput:\n%s", err, out)
		}
		assertContains(t, out, "References found for User.age")
		assertContains(t, out, "app/user.rb")
	})

	t.Run("RemovedColumnNotFound", func(t *testing.T) {
		// legacy_token was created in create_table but removed by remove_column.
		out, err := run(t, "check", "--orm", "rails", "--model", "User", "--field", "legacy_token", fixture)
		if err == nil {
			t.Fatal("expected non-zero exit for removed column")
		}
		assertContains(t, out, `"legacy_token" not found`)
	})
}

func TestE2E_UnknownOrm(t *testing.T) {
	out, err := run(t, "check", "--orm", "typeorm", "--model", "User", "--field", "email", "fixtures/django")
	if err == nil {
		t.Fatal("expected non-zero exit for unknown --orm")
	}
	assertContains(t, out, `unknown --orm "typeorm"`)
	assertContains(t, out, "supported values are django, rails")
}

func TestE2E_MissingFlags(t *testing.T) {
	t.Run("MissingOrm", func(t *testing.T) {
		out, err := run(t, "check", "--model", "User", "--field", "email")
		if err == nil {
			t.Fatal("expected non-zero exit for missing --orm")
		}
		assertContains(t, out, "orm")
	})

	t.Run("MissingModel", func(t *testing.T) {
		out, err := run(t, "check", "--orm", "django", "--field", "email")
		if err == nil {
			t.Fatal("expected non-zero exit for missing --model")
		}
		assertContains(t, out, "model")
	})

	t.Run("MissingField", func(t *testing.T) {
		out, err := run(t, "check", "--orm", "django", "--model", "User")
		if err == nil {
			t.Fatal("expected non-zero exit for missing --field")
		}
		assertContains(t, out, "field")
	})
}

func TestE2E_PatternBattery_Django(t *testing.T) {
	runPatternBattery(t, "django", "../test_patterns/django/golden_title.txt",
		[]string{"../test_patterns/django/references.py"},
		"check", "--orm", "django", "--model", "Article", "--field", "title", "../test_patterns/django")
}

func TestE2E_PatternBattery_Rails(t *testing.T) {
	runPatternBattery(t, "rails", "../test_patterns/rails/golden_title.txt",
		[]string{"../test_patterns/rails/references.rb"},
		"check", "--orm", "rails", "--model", "Article", "--field", "title", "../test_patterns/rails")
}

func runPatternBattery(t *testing.T, name, goldenPath string, noRefFiles []string, args ...string) {
	t.Helper()
	out, err := run(t, args...)
	if err != nil {
		t.Fatalf("unexpected error: %v\noutput:\n%s", err, out)
	}
	if *update {
		if err := os.WriteFile(goldenPath, out, 0o644); err != nil {
			t.Fatalf("failed to write golden file: %v", err)
		}
		t.Logf("updated golden file: %s", goldenPath)
		return
	}
	golden, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("failed to read golden file: %v", err)
	}
	if !bytes.Equal(out, golden) {
		t.Errorf("%s output differs from golden — run 'make update-golden' to refresh\ngot:\n%s\nwant:\n%s", name, out, golden)
	}
	assertNoRefs(t, out, noRefFiles)
}

// assertNoRefs scans srcFiles for lines marked [no-ref] and asserts none of
// those line numbers appear in the colref output.
func assertNoRefs(t *testing.T, out []byte, srcFiles []string) {
	t.Helper()
	for _, srcFile := range srcFiles {
		f, err := os.Open(srcFile)
		if err != nil {
			t.Fatalf("assertNoRefs: cannot open %s: %v", srcFile, err)
		}
		base := filepath.Base(srcFile)
		lineNum := 0
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			lineNum++
			if strings.Contains(scanner.Text(), "[no-ref]") {
				needle := fmt.Sprintf("%s:%d", base, lineNum)
				if bytes.Contains(out, []byte(needle)) {
					t.Errorf("[no-ref] line was detected but should not be: %s", needle)
				}
			}
		}
		if err := f.Close(); err != nil {
			t.Fatalf("assertNoRefs: cannot close %s: %v", srcFile, err)
		}
	}
}

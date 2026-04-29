package e2e

import (
	"bytes"
	"os"
	"os/exec"
	"testing"
)

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
	})

	t.Run("NoRefs", func(t *testing.T) {
		out, err := run(t, "check", "--orm", "django", "--model", "User", "--field", "name", fixture)
		if err != nil {
			t.Fatalf("unexpected error: %v\noutput:\n%s", err, out)
		}
		assertContains(t, out, "No references found for User.name")
		assertContains(t, out, "Verify manually before deleting.")
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
		dir := t.TempDir()
		out, err := run(t, "check", "--orm", "rails", "--model", "User", "--field", "email", dir)
		if err == nil {
			t.Fatal("expected non-zero exit for missing schema.rb")
		}
		assertContains(t, out, "schema.rb")
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

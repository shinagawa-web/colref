package scanner

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestScan_BasicMatch(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "views.py", `
def get_email(user):
    return user.email
`)

	refs, count, err := Scan(dir, "email")
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("want 1 file scanned, got %d", count)
	}
	if len(refs) != 1 {
		t.Fatalf("want 1 ref, got %d: %v", len(refs), refs)
	}
	if refs[0].Text != "user.email" {
		t.Errorf("want text %q, got %q", "user.email", refs[0].Text)
	}
	if refs[0].Line != 3 {
		t.Errorf("want line 3, got %d", refs[0].Line)
	}
}

func TestScan_NestedAttributeAccess(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "views.py", `email = request.user.email`)

	refs, _, err := Scan(dir, "email")
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 1 {
		t.Fatalf("want 1 ref, got %d: %v", len(refs), refs)
	}
	if refs[0].Text != "request.user.email" {
		t.Errorf("want %q, got %q", "request.user.email", refs[0].Text)
	}
}

func TestScan_CommentNotMatched(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "views.py", `# user.email is used here`)

	refs, _, err := Scan(dir, "email")
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 0 {
		t.Errorf("want 0 refs from comment, got %d: %v", len(refs), refs)
	}
}

func TestScan_StringNotMatched(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "views.py", `qs = User.objects.values("email")`)

	refs, _, err := Scan(dir, "email")
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 0 {
		t.Errorf("want 0 refs from string literal, got %d: %v", len(refs), refs)
	}
}

func TestScan_NonMatchingAttribute(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "views.py", `x = user.name`)

	refs, _, err := Scan(dir, "email")
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 0 {
		t.Errorf("want 0 refs, got %d", len(refs))
	}
}

func TestScan_SkipMigrationsDir(t *testing.T) {
	dir := t.TempDir()
	migrationsDir := filepath.Join(dir, "migrations")
	if err := os.MkdirAll(migrationsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, migrationsDir, "0001_initial.py", `x = user.email`)

	refs, count, err := Scan(dir, "email")
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("want 0 files scanned (migrations skipped), got %d", count)
	}
	if len(refs) != 0 {
		t.Errorf("want 0 refs from migrations dir, got %d", len(refs))
	}
}

func TestScan_DotDirAsRoot(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "views.py", `x = user.email`)

	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(orig)

	refs, count, err := Scan(".", "email")
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("want 1 file scanned when dir is \".\", got %d", count)
	}
	if len(refs) != 1 {
		t.Fatalf("want 1 ref, got %d", len(refs))
	}
}

func TestScan_MultipleFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.py", `x = user.email`)
	writeFile(t, dir, "b.py", `y = obj.email`)
	writeFile(t, dir, "c.py", `z = item.name`)

	refs, count, err := Scan(dir, "email")
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("want 3 files scanned, got %d", count)
	}
	if len(refs) != 2 {
		t.Fatalf("want 2 refs, got %d: %v", len(refs), refs)
	}
}

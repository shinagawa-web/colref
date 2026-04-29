package scanner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	sitter "github.com/smacker/go-tree-sitter"
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
	t.Cleanup(func() { _ = os.Chdir(orig) })

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

func TestScan_DuplicateOnSameLine(t *testing.T) {
	dir := t.TempDir()
	// self.email appears twice on the same line (lhs and rhs of assignment).
	writeFile(t, dir, "models.py", `self.email = normalize(self.email)`)

	refs, _, err := Scan(dir, "email")
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 1 {
		t.Fatalf("want 1 ref (deduped), got %d: %v", len(refs), refs)
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

// TestScan_SkipNonPyFiles covers the non-.py file return path.
func TestScan_SkipNonPyFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "views.py", `x = user.email`)
	writeFile(t, dir, "README.txt", `x = user.email`)

	_, count, err := Scan(dir, "email")
	if err != nil {
		t.Fatal(err)
	}
	// Only the .py file should be counted.
	if count != 1 {
		t.Fatalf("want 1 file scanned (only .py), got %d", count)
	}
}

// TestScan_ReadFileError covers the os.ReadFile error path by making a .py
// file unreadable.
func TestScan_ReadFileError(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "views.py")
	if err := os.WriteFile(p, []byte(`x = user.email`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(p, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(p, 0o644) })

	_, _, err := Scan(dir, "email")
	if err == nil {
		t.Fatal("expected error when .py file is unreadable")
	}
}

// TestScan_WalkError covers the WalkDir callback error path by making
// a subdirectory unreadable so the OS passes an error to the callback.
func TestScan_WalkError(t *testing.T) {
	dir := t.TempDir()
	subdir := filepath.Join(dir, "app")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, subdir, "views.py", `x = user.email`)
	// Make the directory unreadable so WalkDir fails when entering it.
	if err := os.Chmod(subdir, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(subdir, 0o755) })

	_, _, err := Scan(dir, "email")
	if err == nil {
		t.Fatal("expected error when subdirectory is unreadable")
	}
}

// TestScan_ParseCtxError covers the ParseCtx error path by injecting a failing
// parseCtxFn.
func TestScan_ParseCtxError(t *testing.T) {
	orig := parseCtxFn
	parseCtxFn = func(p *sitter.Parser, ctx context.Context, oldTree *sitter.Tree, src []byte) (*sitter.Tree, error) {
		return nil, context.Canceled
	}
	defer func() { parseCtxFn = orig }()

	dir := t.TempDir()
	writeFile(t, dir, "views.py", `x = user.email`)

	_, _, err := Scan(dir, "email")
	if err == nil {
		t.Fatal("expected error when parseCtxFn fails")
	}
}

// TestScan_FilepathRelError covers the filepath.Rel error path by injecting a
// failing filepathRelFn.
func TestScan_FilepathRelError(t *testing.T) {
	orig := filepathRelFn
	filepathRelFn = func(basepath, targpath string) (string, error) {
		return "", fmt.Errorf("injected Rel error")
	}
	defer func() { filepathRelFn = orig }()

	dir := t.TempDir()
	writeFile(t, dir, "views.py", `x = user.email`)

	// Should not error overall — the fallback uses filepath.Clean(path).
	refs, count, err := Scan(dir, "email")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 1 {
		t.Fatalf("want 1 file scanned, got %d", count)
	}
	if len(refs) != 1 {
		t.Fatalf("want 1 ref, got %d: %v", len(refs), refs)
	}
}

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

func TestScan_MultiLineChain_LineNumber(t *testing.T) {
	dir := t.TempDir()
	// .email is on line 3, not line 1 where the chain starts.
	writeFile(t, dir, "views.py", `result = (
    obj.get()
    .email
)`)

	refs, _, err := Scan(dir, "email")
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 1 {
		t.Fatalf("want 1 ref, got %d: %v", len(refs), refs)
	}
	if refs[0].Line != 3 {
		t.Errorf("want line 3 (where .email appears), got %d", refs[0].Line)
	}
	if refs[0].Text != ".email" {
		t.Errorf("want text %q, got %q", ".email", refs[0].Text)
	}
}

// --- attribute-access patterns that must be detected ---

func TestScan_FString(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "views.py", `msg = f"contact: {user.email}"`)

	refs, _, err := Scan(dir, "email")
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 1 {
		t.Fatalf("want 1 ref inside f-string, got %d: %v", len(refs), refs)
	}
}

func TestScan_Lambda(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "utils.py", `get_email = lambda u: u.email`)

	refs, _, err := Scan(dir, "email")
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 1 {
		t.Fatalf("want 1 ref in lambda, got %d: %v", len(refs), refs)
	}
}

func TestScan_ListComprehension(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "views.py", `emails = [u.email for u in users]`)

	refs, _, err := Scan(dir, "email")
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 1 {
		t.Fatalf("want 1 ref in list comprehension, got %d: %v", len(refs), refs)
	}
}

func TestScan_ChainedCall(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "views.py", `addr = user.email.lower()`)

	refs, _, err := Scan(dir, "email")
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 1 {
		t.Fatalf("want 1 ref (chained call), got %d: %v", len(refs), refs)
	}
}

func TestScan_SelfReference(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "models.py", `
class User:
    def clean(self):
        self.email = self.email.strip()
`)

	refs, _, err := Scan(dir, "email")
	if err != nil {
		t.Fatal(err)
	}
	// Two attribute accesses on same line → deduplicated to 1.
	if len(refs) != 1 {
		t.Fatalf("want 1 ref (self, deduped), got %d: %v", len(refs), refs)
	}
}

func TestScan_WalrusOperator(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "views.py", `
if addr := user.email:
    send(addr)
`)

	refs, _, err := Scan(dir, "email")
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 1 {
		t.Fatalf("want 1 ref with walrus operator, got %d: %v", len(refs), refs)
	}
}

func TestScan_TernaryExpression(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "views.py", `addr = user.email if user.is_active else None`)

	refs, _, err := Scan(dir, "email")
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 1 {
		t.Fatalf("want 1 ref in ternary, got %d: %v", len(refs), refs)
	}
}

func TestScan_TypeAnnotatedFunction(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "views.py", `
def get_email(user: User) -> str:
    return user.email
`)

	refs, _, err := Scan(dir, "email")
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 1 {
		t.Fatalf("want 1 ref in type-annotated function, got %d: %v", len(refs), refs)
	}
}

// --- v0.1 known limitations: string-based ORM calls are NOT detected ---
// These tests document the current scope boundary described in the README.

func TestScan_FilterKwarg_NotDetected(t *testing.T) {
	dir := t.TempDir()
	// v0.1 limitation: keyword argument in ORM filter is a string key, not attribute access.
	writeFile(t, dir, "views.py", `qs = User.objects.filter(email="x@example.com")`)

	refs, _, err := Scan(dir, "email")
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 0 {
		t.Errorf("filter kwarg should not be detected in v0.1, got %d: %v", len(refs), refs)
	}
}

func TestScan_ValuesListString_NotDetected(t *testing.T) {
	dir := t.TempDir()
	// v0.1 limitation: string argument to .values_list() is not attribute access.
	writeFile(t, dir, "views.py", `qs = User.objects.values_list("email", flat=True)`)

	refs, _, err := Scan(dir, "email")
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 0 {
		t.Errorf(".values_list() string should not be detected in v0.1, got %d: %v", len(refs), refs)
	}
}

func TestScan_QObject_NotDetected(t *testing.T) {
	dir := t.TempDir()
	// v0.1 limitation: Q(email=...) passes the column as a keyword argument string.
	writeFile(t, dir, "views.py", `from django.db.models import Q; qs = User.objects.filter(Q(email="x"))`)

	refs, _, err := Scan(dir, "email")
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 0 {
		t.Errorf("Q() kwarg should not be detected in v0.1, got %d: %v", len(refs), refs)
	}
}

func TestScan_GetAttr_NotDetected(t *testing.T) {
	dir := t.TempDir()
	// v0.1 limitation: getattr() takes the field name as a string.
	writeFile(t, dir, "views.py", `v = getattr(user, "email")`)

	refs, _, err := Scan(dir, "email")
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 0 {
		t.Errorf("getattr() should not be detected in v0.1, got %d: %v", len(refs), refs)
	}
}

func TestLineAt_OutOfBounds(t *testing.T) {
	lines := [][]byte{[]byte("line0"), []byte("line1")}
	if got := lineAt(lines, 5); got != "" {
		t.Errorf("want empty string for out-of-bounds row, got %q", got)
	}
}

func TestPythonScanner_Methods(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "views.py", `x = user.email`)

	s := PythonScanner{}

	refs, count, err := s.Scan(dir, "email")
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 || len(refs) != 1 {
		t.Errorf("unexpected result: count=%d refs=%v", count, refs)
	}

	skipDirs := s.SkipDirs()
	if !skipDirs["migrations"] {
		t.Error("expected migrations to be in SkipDirs")
	}
}

func TestScan_FExpression_NotDetected(t *testing.T) {
	dir := t.TempDir()
	// v0.1 limitation: F('email') passes the column as a string.
	writeFile(t, dir, "views.py", `from django.db.models import F; User.objects.update(email=F("email"))`)

	refs, _, err := Scan(dir, "email")
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 0 {
		t.Errorf("F() expression should not be detected in v0.1, got %d: %v", len(refs), refs)
	}
}

func TestScanRuby_BasicMatch(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "app.rb", `
def show
  render json: user.email
end
`)

	refs, count, err := ScanRuby(dir, "email")
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

func TestScanRuby_SkipNonRbFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "views.py", `return user.email`)

	refs, count, err := ScanRuby(dir, "email")
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("want 0 .rb files scanned, got %d", count)
	}
	if len(refs) != 0 {
		t.Errorf("want no refs from .py file, got %v", refs)
	}
}

func TestScanRuby_MultiLine(t *testing.T) {
	dir := t.TempDir()
	// Method on a different line than the receiver.
	writeFile(t, dir, "app.rb", "user\n  .email\n")

	refs, _, err := ScanRuby(dir, "email")
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 1 {
		t.Fatalf("want 1 ref for multi-line chain, got %d: %v", len(refs), refs)
	}
	if refs[0].Line != 2 {
		t.Errorf("want line 2 (method line), got %d", refs[0].Line)
	}
	if refs[0].Text != ".email" {
		t.Errorf("want trimmed method line %q, got %q", ".email", refs[0].Text)
	}
}

func TestScanRuby_NoMatch(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "app.rb", `user.name`)

	refs, _, err := ScanRuby(dir, "email")
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 0 {
		t.Errorf("want no refs, got %v", refs)
	}
}

func TestRubyScanner_Methods(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "app.rb", `user.email`)

	s := RubyScanner{}

	refs, count, err := s.Scan(dir, "email")
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 || len(refs) != 1 {
		t.Errorf("unexpected result: count=%d refs=%v", count, refs)
	}

	skipDirs := s.SkipDirs()
	if !skipDirs["node_modules"] {
		t.Error("expected node_modules to be in RubySkipDirs")
	}
	if !skipDirs["spec"] {
		t.Error("expected spec to be in RubySkipDirs")
	}
	if skipDirs["migrations"] {
		t.Error("migrations should not be in RubySkipDirs")
	}
}

func TestScanRuby_SkipSpecDir(t *testing.T) {
	dir := t.TempDir()
	specDir := filepath.Join(dir, "spec")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, specDir, "user_spec.rb", `user.email`)

	refs, count, err := ScanRuby(dir, "email")
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("want 0 files scanned (spec skipped), got %d", count)
	}
	if len(refs) != 0 {
		t.Errorf("want 0 refs from spec dir, got %d", len(refs))
	}
}

func TestScanRuby_SkipTestDir(t *testing.T) {
	dir := t.TempDir()
	testDir := filepath.Join(dir, "test")
	if err := os.MkdirAll(testDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, testDir, "user_test.rb", `user.email`)

	refs, count, err := ScanRuby(dir, "email")
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("want 0 files scanned (test skipped), got %d", count)
	}
	if len(refs) != 0 {
		t.Errorf("want 0 refs from test dir, got %d", len(refs))
	}
}

func TestScanRuby_SkipVendorDir(t *testing.T) {
	dir := t.TempDir()
	vendorDir := filepath.Join(dir, "vendor")
	if err := os.MkdirAll(vendorDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, vendorDir, "lib.rb", `user.email`)

	refs, count, err := ScanRuby(dir, "email")
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("want 0 files scanned (vendor skipped), got %d", count)
	}
	if len(refs) != 0 {
		t.Errorf("want 0 refs from vendor dir, got %d", len(refs))
	}
}

func TestScanRuby_SkipMigrateDir(t *testing.T) {
	dir := t.TempDir()
	migrateDir := filepath.Join(dir, "migrate")
	if err := os.MkdirAll(migrateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, migrateDir, "001_create_users.rb", `user.email`)

	refs, count, err := ScanRuby(dir, "email")
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("want 0 files scanned (migrate skipped), got %d", count)
	}
	if len(refs) != 0 {
		t.Errorf("want 0 refs from migrate dir, got %d", len(refs))
	}
}

func TestScanRuby_ERBBasicMatch(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "user.html.erb", `<%= user.email %>`)

	refs, count, err := ScanRuby(dir, "email")
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("want 1 file scanned, got %d", count)
	}
	if len(refs) != 1 {
		t.Fatalf("want 1 ref, got %d: %v", len(refs), refs)
	}
}

func TestScanRuby_ERBInstanceVar(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "user.html.erb", `<%= @user.email %>`)

	refs, count, err := ScanRuby(dir, "email")
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("want 1 file scanned, got %d", count)
	}
	if len(refs) != 1 {
		t.Fatalf("want 1 ref, got %d: %v", len(refs), refs)
	}
}

func TestScanRuby_ERBNonOutputTag(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "user.html.erb", `<% user.email %>`)

	refs, count, err := ScanRuby(dir, "email")
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("want 1 file scanned, got %d", count)
	}
	if len(refs) != 1 {
		t.Fatalf("want 1 ref, got %d: %v", len(refs), refs)
	}
}

func TestScanRuby_ERBLineNumber(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "user.html.erb", "<h1>Title</h1>\n<p><%= @user.email %></p>")

	refs, _, err := ScanRuby(dir, "email")
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 1 {
		t.Fatalf("want 1 ref, got %d: %v", len(refs), refs)
	}
	if refs[0].Line != 2 {
		t.Errorf("want line 2, got %d", refs[0].Line)
	}
}

func TestScanRuby_ERBNoMatch(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "user.html.erb", `<%= user.name %>`)

	refs, _, err := ScanRuby(dir, "email")
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 0 {
		t.Errorf("want 0 refs, got %d: %v", len(refs), refs)
	}
}

func TestScanRuby_ERBComment(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "user.html.erb", `<%# user.email %>`)

	refs, _, err := ScanRuby(dir, "email")
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 0 {
		t.Errorf("want 0 refs from ERB comment, got %d: %v", len(refs), refs)
	}
}

func TestScanRuby_ERBCommentThenExpression(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "user.html.erb", `<%# x %><%= user.email %>`)

	refs, _, err := ScanRuby(dir, "email")
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 1 {
		t.Fatalf("want 1 ref after comment, got %d: %v", len(refs), refs)
	}
}

func TestScanRuby_ERBDashClose(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "user.html.erb", `<%= @user.email -%>`)

	refs, _, err := ScanRuby(dir, "email")
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 1 {
		t.Fatalf("want 1 ref with -%%> closing, got %d: %v", len(refs), refs)
	}
}

func TestScanRuby_ERBCountedInFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "app.rb", `user.email`)
	writeFile(t, dir, "user.html.erb", `<%= user.email %>`)

	_, count, err := ScanRuby(dir, "email")
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("want 2 files scanned (.rb + .erb), got %d", count)
	}
}

func TestScanRuby_ERBParseCtxError(t *testing.T) {
	orig := parseCtxFn
	parseCtxFn = func(p *sitter.Parser, ctx context.Context, oldTree *sitter.Tree, src []byte) (*sitter.Tree, error) {
		return nil, context.Canceled
	}
	defer func() { parseCtxFn = orig }()

	dir := t.TempDir()
	writeFile(t, dir, "user.html.erb", `<%= user.email %>`)

	_, _, err := ScanRuby(dir, "email")
	if err == nil {
		t.Fatal("expected error when parseCtxFn fails on .erb file")
	}
}

func TestScanRuby_RbParseCtxError(t *testing.T) {
	orig := parseCtxFn
	parseCtxFn = func(p *sitter.Parser, ctx context.Context, oldTree *sitter.Tree, src []byte) (*sitter.Tree, error) {
		return nil, context.Canceled
	}
	defer func() { parseCtxFn = orig }()

	dir := t.TempDir()
	writeFile(t, dir, "app.rb", `user.email`)

	_, _, err := ScanRuby(dir, "email")
	if err == nil {
		t.Fatal("expected error when parseCtxFn fails on .rb file")
	}
}

func TestERBToRuby(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "html replaced with spaces, newlines preserved",
			input: "<h1>hello</h1>\n",
			want:  "              \n",
		},
		{
			name:  "output tag content preserved",
			input: "<%= user.email %>",
			want:  "    user.email   ",
		},
		{
			name:  "non-output tag content preserved",
			input: "<% user.email %>",
			want:  "   user.email   ",
		},
		{
			name:  "dash modifier opening",
			input: "<%= @user.email -%>",
			want:  "    @user.email -  ",
		},
		{
			name:  "dash modifier in opening tag (<%- )",
			input: "<%- user.email %>",
			want:  "    user.email   ",
		},
		{
			name:  "comment tag: content replaced with spaces",
			input: "<%# user.email %>",
			want:  "                 ",
		},
		{
			name:  "comment tag preserves newlines",
			input: "<%# comment\nnext %>",
			want:  "           \n       ",
		},
		{
			name:  "comment then expression",
			input: "<%# x %><%= user.email %>",
			want:  "            user.email   ",
		},
		{
			name:  "lone < not followed by % is replaced with space",
			input: "<b>text</b>",
			want:  "           ",
		},
		{
			name:  "ruby mode: lone % not followed by > is preserved",
			input: "<% x = 5 % 2 %>",
			want:  "   x = 5 % 2   ",
		},
		{
			name:  "empty input",
			input: "",
			want:  "",
		},
		{
			name:  "newline in html mode",
			input: "hello\nworld",
			want:  "     \n     ",
		},
		{
			name:  "double-quoted string: %> inside is not a closing tag",
			input: `<%= "%>" %>`,
			want:  `    "%>"   `,
		},
		{
			name:  "single-quoted string: %> inside is not a closing tag",
			input: `<%= '%>' %>`,
			want:  `    '%>'   `,
		},
		{
			name:  "escaped quote inside string does not close string early",
			input: "<%= \"a\\\"b\" %>",
			want:  "    \"a\\\"b\"   ",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := string(erbToRuby([]byte(tc.input)))
			if got != tc.want {
				t.Errorf("erbToRuby(%q)\nwant: %q\n got: %q", tc.input, tc.want, got)
			}
		})
	}
}

func TestScanRuby_ERBStringLiteralWithPercentGT(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "show.html.erb", `<%= "%>" %>`)

	refs, _, err := ScanRuby(dir, "email")
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 0 {
		t.Errorf("want 0 refs (string literal contains %%>, not a call), got %d: %v", len(refs), refs)
	}
}

// BenchmarkScan uses 1,000 Python files (200 apps × 5 files, ~100 lines each) with
// per-app generated names — comparable to BookWyrm scale (~433 files, ~52k lines)
// in line density while exceeding it in file count.
func BenchmarkScan(b *testing.B) {
	dir := b.TempDir()

	const viewsFmt = `
from django.shortcuts import get_object_or_404, render
from django.http import JsonResponse
from django.views import View
from django.contrib.auth.decorators import login_required

def list_%[1]d(request):
    qs = User.objects.filter(is_active=True).select_related("profile")
    return JsonResponse([u.email for u in qs], safe=False)

def detail_%[1]d(request, pk):
    user = get_object_or_404(User, pk=pk)
    return JsonResponse({"id": user.pk, "email": user.email, "name": user.display_name})

def create_%[1]d(request):
    email = request.POST.get("email")
    if User.objects.filter(email=email).exists():
        return JsonResponse({"error": "email taken"}, status=400)
    user = User.objects.create(email=email, display_name=request.POST.get("name", ""))
    return JsonResponse({"id": user.pk, "email": user.email}, status=201)

def update_%[1]d(request, pk):
    user = get_object_or_404(User, pk=pk)
    user.display_name = request.POST.get("name", user.display_name)
    user.save()
    return JsonResponse({"email": user.email, "name": user.display_name})

def delete_%[1]d(request, pk):
    user = get_object_or_404(User, pk=pk)
    email = user.email
    user.delete()
    return JsonResponse({"deleted": email})

@login_required
def profile_%[1]d(request):
    return render(request, "profile.html", {
        "email": request.user.email,
        "name": request.user.display_name,
    })

class App%[1]dListView(View):
    def get(self, request):
        users = User.objects.filter(is_active=True).order_by("-created_at")
        data = [{"id": u.pk, "email": u.email, "name": u.display_name} for u in users]
        return JsonResponse({"results": data, "count": len(data)})

class App%[1]dDetailView(View):
    def get(self, request, pk):
        user = get_object_or_404(User, pk=pk)
        return JsonResponse({"id": user.pk, "email": user.email, "verified": user.is_verified})

    def patch(self, request, pk):
        user = get_object_or_404(User, pk=pk)
        if name := request.POST.get("name"):
            user.display_name = name
        user.save()
        return JsonResponse({"email": user.email})

    def delete(self, request, pk):
        user = get_object_or_404(User, pk=pk)
        email = user.email
        user.delete()
        return JsonResponse({"deleted": email})

class App%[1]dCreateView(View):
    def post(self, request):
        email = request.POST["email"]
        if User.objects.filter(email=email).exists():
            return JsonResponse({"error": "email taken"}, status=400)
        user = User.objects.create(email=email, display_name=request.POST.get("name", ""))
        return JsonResponse({"id": user.pk, "email": user.email}, status=201)

class App%[1]dUpdateView(View):
    def post(self, request, pk):
        user = get_object_or_404(User, pk=pk)
        user.display_name = request.POST.get("name", user.display_name)
        user.bio = request.POST.get("bio", user.bio)
        user.save()
        return JsonResponse({"email": user.email, "name": user.display_name})

class App%[1]dDeleteView(View):
    def post(self, request, pk):
        user = get_object_or_404(User, pk=pk)
        email = user.email
        user.delete()
        return JsonResponse({"deleted": email})
`

	const serializersFmt = `
class App%[1]dUserSerializer:
    def to_representation(self, instance):
        return {
            "id": instance.pk,
            "email": instance.email,
            "name": instance.display_name,
            "verified": instance.is_verified,
            "bio": instance.bio,
        }

    def validate_email(self, value):
        if User.objects.filter(email=value).exists():
            raise ValueError("email already in use")
        return value

    def validate_display_name(self, value):
        if len(value) < 2:
            raise ValueError("display_name too short")
        return value

class App%[1]dDetailSerializer(App%[1]dUserSerializer):
    def to_representation(self, instance):
        data = super().to_representation(instance)
        data["avatar"] = instance.avatar.url if instance.avatar else None
        return data

class App%[1]dListSerializer(App%[1]dUserSerializer):
    def to_representation(self, instance):
        return {"id": instance.pk, "email": instance.email, "name": instance.display_name}

class App%[1]dCreateSerializer:
    def validate(self, data):
        if not data.get("email"):
            raise ValueError("email required")
        if User.objects.filter(email=data["email"]).exists():
            raise ValueError("email taken")
        return data

    def create(self, validated_data):
        user = User(**validated_data)
        user.save()
        return user

class App%[1]dUpdateSerializer:
    def update(self, instance, validated_data):
        instance.display_name = validated_data.get("display_name", instance.display_name)
        instance.bio = validated_data.get("bio", instance.bio)
        instance.save()
        return instance

    def to_representation(self, instance):
        return {"email": instance.email, "name": instance.display_name}
`

	const tasksFmt = `
from celery import shared_task

@shared_task
def send_welcome_%[1]d(user_id):
    user = User.objects.get(pk=user_id)
    send_mail(subject="Welcome", message=f"Hi {user.display_name}", recipient_list=[user.email])

@shared_task
def send_notification_%[1]d(user_id, message):
    user = User.objects.get(pk=user_id)
    if user.is_verified:
        notify(user.email, message)

@shared_task
def deactivate_unverified_%[1]d():
    for user in User.objects.filter(is_verified=False):
        log(f"deactivating {user.email}")
        user.is_active = False
        user.save()

@shared_task
def send_reset_%[1]d(user_id):
    user = User.objects.get(pk=user_id)
    token = generate_token(user.pk)
    send_mail(subject="Reset", message=f"Reset for {user.email}: /reset/{token}", recipient_list=[user.email])

@shared_task
def send_verification_%[1]d(user_id):
    user = User.objects.get(pk=user_id)
    if not user.is_verified:
        code = generate_code(user.pk)
        send_mail(subject="Verify", message=f"Verify {user.email}: /verify/{code}", recipient_list=[user.email])

@shared_task
def sync_to_crm_%[1]d(user_id):
    user = User.objects.get(pk=user_id)
    crm.upsert({"email": user.email, "name": user.display_name, "verified": user.is_verified})

@shared_task
def cleanup_inactive_%[1]d():
    for user in User.objects.filter(is_active=False):
        log(f"removing {user.email}")
        user.delete()
`

	const adminFmt = `
from django.contrib import admin
from django.utils.html import format_html

@admin.register(User)
class App%[1]dUserAdmin(admin.ModelAdmin):
    list_display = ["email", "display_name", "is_verified", "is_active", "created_at"]
    list_filter = ["is_verified", "is_active", "created_at"]
    search_fields = ["email", "display_name"]
    readonly_fields = ["created_at", "updated_at"]
    ordering = ["-created_at"]

    def get_queryset(self, request):
        return super().get_queryset(request).select_related()

    def email_link(self, obj):
        return format_html('<a href="mailto:{}">{}</a>', obj.email, obj.email)
    email_link.short_description = "Email"

    def verify_users(self, request, queryset):
        updated = queryset.update(is_verified=True)
        self.message_user(request, f"{updated} users verified.")
    verify_users.short_description = "Mark selected users as verified"

    def deactivate_users(self, request, queryset):
        for user in queryset:
            log(f"admin deactivated {user.email}")
        queryset.update(is_active=False)
    deactivate_users.short_description = "Deactivate selected users"

    actions = ["verify_users", "deactivate_users"]

@admin.register(Organisation)
class App%[1]dOrgAdmin(admin.ModelAdmin):
    list_display = ["name", "slug", "owner_email", "created_at"]
    search_fields = ["name", "slug"]
    readonly_fields = ["created_at"]

    def owner_email(self, obj):
        return obj.owner.email
    owner_email.short_description = "Owner"
`

	const signalsFmt = `
from django.db.models.signals import post_save, pre_delete, post_delete
from django.dispatch import receiver

@receiver(post_save, sender=User)
def on_created_%[1]d(sender, instance, created, **kwargs):
    if created:
        send_welcome_%[1]d.delay(instance.pk)
        log(f"new user: {instance.email}")

@receiver(post_save, sender=User)
def on_updated_%[1]d(sender, instance, created, **kwargs):
    if not created and instance.is_verified:
        cache.set(f"user:{instance.pk}:email", instance.email)

@receiver(post_save, sender=User)
def on_verified_%[1]d(sender, instance, created, **kwargs):
    if not created and instance.is_verified:
        send_mail("Verified", f"Account {instance.email} verified.", [instance.email])

@receiver(pre_delete, sender=User)
def on_pre_delete_%[1]d(sender, instance, **kwargs):
    log(f"about to delete: {instance.email}")
    cache.delete(f"user:{instance.pk}:email")

@receiver(post_delete, sender=User)
def on_deleted_%[1]d(sender, instance, **kwargs):
    log(f"deleted: {instance.email}")
    audit_log.record("delete", "User", instance.email)

@receiver(post_save, sender=Organisation)
def on_org_created_%[1]d(sender, instance, created, **kwargs):
    if created:
        notify(instance.owner.email, f"Org '{instance.name}' created")
`

	formats := []struct {
		suffix string
		format string
	}{
		{"views", viewsFmt},
		{"serializers", serializersFmt},
		{"tasks", tasksFmt},
		{"admin", adminFmt},
		{"signals", signalsFmt},
	}

	for i := 0; i < 200; i++ {
		appDir := filepath.Join(dir, fmt.Sprintf("app%03d", i))
		if err := os.MkdirAll(appDir, 0o755); err != nil {
			b.Fatal(err)
		}
		for _, f := range formats {
			content := fmt.Sprintf(f.format, i)
			path := filepath.Join(appDir, f.suffix+".py")
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				b.Fatal(err)
			}
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, _, err := Scan(dir, "email"); err != nil {
			b.Fatal(err)
		}
	}
}

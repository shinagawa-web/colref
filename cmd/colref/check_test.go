package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shinagawa-web/colref/internal/parser"
)

func setupDjangoFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	accounts := filepath.Join(dir, "accounts")
	blog := filepath.Join(dir, "blog")
	if err := os.MkdirAll(accounts, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(blog, 0o755); err != nil {
		t.Fatal(err)
	}

	write := func(path, content string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	write(filepath.Join(accounts, "models.py"), `
from django.db import models

class User(models.Model):
    email = models.EmailField()
    name = models.CharField(max_length=100)
`)
	write(filepath.Join(blog, "models.py"), `
from django.db import models

class Post(models.Model):
    title = models.CharField(max_length=200)
`)
	write(filepath.Join(accounts, "views.py"), `
def send(user):
    return user.email
`)
	write(filepath.Join(blog, "views.py"), `
def show(post):
    return post.title
`)
	return dir
}

func setupRailsFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	dbDir := filepath.Join(dir, "db")
	if err := os.MkdirAll(dbDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dbDir, "schema.rb"), []byte(`
ActiveRecord::Schema[7.0].define do
  create_table "users", force: :cascade do |t|
    t.string "email", null: false
    t.string "name"
  end
end
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app.rb"), []byte(`
user.email
`), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestRunCheck_UnknownOrm(t *testing.T) {
	err := runCheck(t.TempDir(), "User", "email", "typeorm")
	if err == nil {
		t.Fatal("expected error for unknown orm")
	}
	if !strings.Contains(err.Error(), "unknown --orm") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunCheck_Django_RefsFound(t *testing.T) {
	dir := setupDjangoFixture(t)
	if err := runCheck(dir, "User", "email", "django"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunCheck_Django_NoRefs(t *testing.T) {
	dir := setupDjangoFixture(t)
	if err := runCheck(dir, "User", "name", "django"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunCheck_Django_UnknownModel(t *testing.T) {
	dir := setupDjangoFixture(t)
	err := runCheck(dir, "Invoice", "amount", "django")
	if err == nil {
		t.Fatal("expected error for unknown model")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention 'not found', got: %v", err)
	}
	if !strings.Contains(err.Error(), "User") && !strings.Contains(err.Error(), "Post") {
		t.Errorf("error should list available models, got: %v", err)
	}
}

func TestRunCheck_Django_UnknownField(t *testing.T) {
	dir := setupDjangoFixture(t)
	err := runCheck(dir, "User", "emial", "django")
	if err == nil {
		t.Fatal("expected error for unknown field")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention 'not found', got: %v", err)
	}
	if !strings.Contains(err.Error(), "email") {
		t.Errorf("error should list available fields, got: %v", err)
	}
}

func TestRunCheck_Django_Conflict(t *testing.T) {
	dir := t.TempDir()
	for _, app := range []string{"app1", "app2"} {
		d := filepath.Join(dir, app)
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "models.py"), []byte(`
from django.db import models
class User(models.Model):
    email = models.EmailField()
`), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	err := runCheck(dir, "User", "email", "django")
	if err == nil {
		t.Fatal("expected conflict error")
	}
	if !strings.Contains(err.Error(), "multiple files") {
		t.Errorf("error should mention 'multiple files', got: %v", err)
	}
	if strings.Contains(err.Error(), dir) {
		t.Errorf("conflict paths should be relative, got: %v", err)
	}
}

func TestRunCheck_Django_NoModelsFile(t *testing.T) {
	dir := t.TempDir()
	err := runCheck(dir, "User", "email", "django")
	if err == nil {
		t.Fatal("expected error when no models.py found")
	}
	if !strings.Contains(err.Error(), "no models.py") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunCheck_Django_NoModelsDetected(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "models.py"), []byte("# no models here\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := runCheck(dir, "User", "email", "django")
	if err == nil {
		t.Fatal("expected error when no models are detected")
	}
	if !strings.Contains(err.Error(), "no models detected") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunCheck_Django_FindModelsFilesWalkError(t *testing.T) {
	err := runCheck("/nonexistent/dir/that/does/not/exist", "User", "email", "django")
	if err == nil {
		t.Fatal("expected error for non-existent directory")
	}
}

func TestRunCheck_Django_FindModelsFilesSkipHiddenDir(t *testing.T) {
	dir := t.TempDir()
	hiddenDir := filepath.Join(dir, ".hidden")
	if err := os.MkdirAll(hiddenDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hiddenDir, "models.py"), []byte(`
from django.db import models
class User(models.Model):
    email = models.EmailField()
`), 0o644); err != nil {
		t.Fatal(err)
	}
	err := runCheck(dir, "User", "email", "django")
	if err == nil {
		t.Fatal("expected error: models.py in hidden dir should be skipped")
	}
	if !strings.Contains(err.Error(), "no models.py") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunCheck_Django_ReadFileError(t *testing.T) {
	dir := t.TempDir()
	modelsPath := filepath.Join(dir, "models.py")
	if err := os.WriteFile(modelsPath, []byte(`
from django.db import models
class User(models.Model):
    email = models.EmailField()
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(modelsPath, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(modelsPath, 0o644) })

	err := runCheck(dir, "User", "email", "django")
	if err == nil {
		t.Fatal("expected error when models.py is unreadable")
	}
}

func TestRunCheck_Django_ScanError(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "models.py"), []byte(`
from django.db import models
class User(models.Model):
    email = models.EmailField()
`), 0o644); err != nil {
		t.Fatal(err)
	}
	unreadable := filepath.Join(dir, "views.py")
	if err := os.WriteFile(unreadable, []byte(`x = user.email`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(unreadable, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(unreadable, 0o644) })

	err := runCheck(dir, "User", "email", "django")
	if err == nil {
		t.Fatal("expected error when .py file is unreadable")
	}
}

func TestRunCheck_Django_ParseModelsError(t *testing.T) {
	origParseModels := parseModels
	parseModels = func(src []byte) ([]parser.Field, error) {
		return nil, fmt.Errorf("injected parse error")
	}
	defer func() { parseModels = origParseModels }()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "models.py"), []byte("class Foo: pass"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := runCheck(dir, "Foo", "bar", "django")
	if err == nil {
		t.Fatal("expected error from ParseModels injection")
	}
	if !strings.Contains(err.Error(), "injected parse error") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunCheck_Django_FilepathRelError(t *testing.T) {
	origRelFn := filepathRelFn
	filepathRelFn = func(basepath, targpath string) (string, error) {
		return "", fmt.Errorf("injected Rel error")
	}
	defer func() { filepathRelFn = origRelFn }()

	dir := t.TempDir()
	for _, app := range []string{"app1", "app2"} {
		d := filepath.Join(dir, app)
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "models.py"), []byte(`
from django.db import models
class User(models.Model):
    email = models.EmailField()
`), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	err := runCheck(dir, "User", "email", "django")
	if err == nil {
		t.Fatal("expected conflict error")
	}
	if !strings.Contains(err.Error(), "multiple files") {
		t.Errorf("expected conflict error, got: %v", err)
	}
}

func TestRunCheck_Rails_RefsFound(t *testing.T) {
	dir := setupRailsFixture(t)
	if err := runCheck(dir, "User", "email", "rails"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunCheck_Rails_NoRefs(t *testing.T) {
	dir := setupRailsFixture(t)
	if err := runCheck(dir, "User", "name", "rails"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunCheck_Rails_UnknownModel(t *testing.T) {
	dir := setupRailsFixture(t)
	err := runCheck(dir, "Invoice", "amount", "rails")
	if err == nil {
		t.Fatal("expected error for unknown model")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention 'not found', got: %v", err)
	}
}

func TestRunCheck_Rails_UnknownField(t *testing.T) {
	dir := setupRailsFixture(t)
	err := runCheck(dir, "User", "nonexistent", "rails")
	if err == nil {
		t.Fatal("expected error for unknown field")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention 'not found', got: %v", err)
	}
}

func TestRunCheck_Rails_NoSchemaFile(t *testing.T) {
	err := runCheck(t.TempDir(), "User", "email", "rails")
	if err == nil {
		t.Fatal("expected error for missing schema.rb")
	}
}

func TestRunCheck_Rails_ParseError(t *testing.T) {
	origParse := parseSchemaRb
	parseSchemaRb = func(src []byte) ([]parser.Field, error) {
		return nil, fmt.Errorf("injected schema parse error")
	}
	defer func() { parseSchemaRb = origParse }()

	dir := t.TempDir()
	dbDir := filepath.Join(dir, "db")
	if err := os.MkdirAll(dbDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dbDir, "schema.rb"), []byte("# schema"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := runCheck(dir, "User", "email", "rails")
	if err == nil {
		t.Fatal("expected parse error")
	}
	if !strings.Contains(err.Error(), "injected schema parse error") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCheckCmd_RunE_WithArg(t *testing.T) {
	dir := setupDjangoFixture(t)

	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{
		"check", dir,
		"--model", "User",
		"--field", "email",
		"--orm", "django",
	})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCheckCmd_RunE_NoArg(t *testing.T) {
	dir := setupDjangoFixture(t)

	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{
		"check",
		"--model", "User",
		"--field", "email",
		"--orm", "django",
	})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

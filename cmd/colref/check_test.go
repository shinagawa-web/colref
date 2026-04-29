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

func setupFixture(t *testing.T) string {
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

func TestRunCheck_RefsFound(t *testing.T) {
	dir := setupFixture(t)
	if err := runCheck(dir, "User", "email", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunCheck_NoRefs(t *testing.T) {
	dir := setupFixture(t)
	if err := runCheck(dir, "User", "name", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunCheck_UnknownModel(t *testing.T) {
	dir := setupFixture(t)
	err := runCheck(dir, "Invoice", "amount", "")
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

func TestRunCheck_UnknownField(t *testing.T) {
	dir := setupFixture(t)
	err := runCheck(dir, "User", "emial", "")
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

func TestRunCheck_Conflict(t *testing.T) {
	dir := t.TempDir()
	apps := []string{"app1", "app2"}
	for _, app := range apps {
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

	err := runCheck(dir, "User", "email", "")
	if err == nil {
		t.Fatal("expected conflict error")
	}
	if !strings.Contains(err.Error(), "multiple files") {
		t.Errorf("error should mention 'multiple files', got: %v", err)
	}
	// Conflict paths should be relative (not absolute).
	if strings.Contains(err.Error(), dir) {
		t.Errorf("conflict paths should be relative, got: %v", err)
	}
}

func TestRunCheck_ModelsFile(t *testing.T) {
	dir := setupFixture(t)
	modelsFile := filepath.Join(dir, "accounts", "models.py")
	if err := runCheck(dir, "User", "email", modelsFile); err != nil {
		t.Fatalf("unexpected error with --models-file: %v", err)
	}
}

func TestRunCheck_NoModelsFile(t *testing.T) {
	dir := t.TempDir()
	err := runCheck(dir, "User", "email", "")
	if err == nil {
		t.Fatal("expected error when no models.py found")
	}
	if !strings.Contains(err.Error(), "no models.py") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestRunCheck_ReadFileError covers the os.ReadFile error path in runCheck
// when --models-file points to a non-existent path.
func TestRunCheck_ReadFileError(t *testing.T) {
	dir := t.TempDir()
	err := runCheck(dir, "User", "email", "/nonexistent/path/models.py")
	if err == nil {
		t.Fatal("expected error when models-file does not exist")
	}
}

// TestRunCheck_NoModelsDetected covers the "no models detected" branch when
// models.py exists but contains no model classes.
func TestRunCheck_NoModelsDetected(t *testing.T) {
	dir := t.TempDir()
	modelsPath := filepath.Join(dir, "models.py")
	// A valid Python file with no model class at all.
	if err := os.WriteFile(modelsPath, []byte("# no models here\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := runCheck(dir, "User", "email", modelsPath)
	if err == nil {
		t.Fatal("expected error when no models are detected")
	}
	if !strings.Contains(err.Error(), "no models detected") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestRunCheck_FindModelsFilesWalkError covers the WalkDir error callback in
// findModelsFiles by passing a non-existent directory (WalkDir fails immediately).
func TestRunCheck_FindModelsFilesWalkError(t *testing.T) {
	err := runCheck("/nonexistent/dir/that/does/not/exist", "User", "email", "")
	if err == nil {
		t.Fatal("expected error for non-existent directory")
	}
}

// TestRunCheck_FindModelsFilesSkipHiddenDir covers the hidden-dir and SkipDirs
// branch in findModelsFiles — models.py inside a hidden dir should not be found.
func TestRunCheck_FindModelsFilesSkipHiddenDir(t *testing.T) {
	dir := t.TempDir()
	// Put a models.py only inside a hidden dir (should be skipped).
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
	// No models.py in the root → should fail with "no models.py found".
	err := runCheck(dir, "User", "email", "")
	if err == nil {
		t.Fatal("expected error: models.py in hidden dir should be skipped")
	}
	if !strings.Contains(err.Error(), "no models.py") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestRunCheck_ScanError covers the scanner.Scan error path by making a .py
// file unreadable after it has been found during the walk.
func TestRunCheck_ScanError(t *testing.T) {
	dir := t.TempDir()
	modelsPath := filepath.Join(dir, "models.py")
	if err := os.WriteFile(modelsPath, []byte(`
from django.db import models
class User(models.Model):
    email = models.EmailField()
`), 0o644); err != nil {
		t.Fatal(err)
	}
	// Create a .py file that scanner will try to read but can't.
	unreadable := filepath.Join(dir, "views.py")
	if err := os.WriteFile(unreadable, []byte(`x = user.email`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(unreadable, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(unreadable, 0o644) })

	err := runCheck(dir, "User", "email", modelsPath)
	if err == nil {
		t.Fatal("expected error when .py file is unreadable")
	}
}

// TestCheckCmd_RunE_WithArg covers the checkCmd RunE closure when a path arg
// is provided (len(args)==1 branch in main.go).
func TestCheckCmd_RunE_WithArg(t *testing.T) {
	dir := setupFixture(t)

	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{
		"check", dir,
		"--model", "User",
		"--field", "email",
	})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestRunCheck_ParseModelsError covers the ParseModels error path by injecting
// a failing parseModels function.
func TestRunCheck_ParseModelsError(t *testing.T) {
	origParseModels := parseModels
	parseModels = func(src []byte) ([]parser.Field, error) {
		return nil, fmt.Errorf("injected parse error")
	}
	defer func() { parseModels = origParseModels }()

	dir := t.TempDir()
	modelsPath := filepath.Join(dir, "models.py")
	if err := os.WriteFile(modelsPath, []byte("class Foo: pass"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := runCheck(dir, "Foo", "bar", modelsPath)
	if err == nil {
		t.Fatal("expected error from ParseModels injection")
	}
	if !strings.Contains(err.Error(), "injected parse error") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestRunCheck_FilepathRelError covers the filepath.Rel error path in the conflict
// check by injecting a failing filepathRelFn.
func TestRunCheck_FilepathRelError(t *testing.T) {
	origRelFn := filepathRelFn
	filepathRelFn = func(basepath, targpath string) (string, error) {
		return "", fmt.Errorf("injected Rel error")
	}
	defer func() { filepathRelFn = origRelFn }()

	// Set up a conflict (same model in two files) to trigger the Rel call.
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

	err := runCheck(dir, "User", "email", "")
	if err == nil {
		t.Fatal("expected conflict error")
	}
	// The error should still mention "multiple files" even with Rel failing.
	if !strings.Contains(err.Error(), "multiple files") {
		t.Errorf("expected conflict error, got: %v", err)
	}
}

// TestCheckCmd_RunE_NoArg covers the checkCmd RunE closure without a path arg
// (default "." branch).
func TestCheckCmd_RunE_NoArg(t *testing.T) {
	dir := setupFixture(t)

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
	})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

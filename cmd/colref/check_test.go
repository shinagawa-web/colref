package main

import (
	"fmt"
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

func TestRunCheck_NoModelsDetected(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "models.py"), []byte(`
class NotAModel:
    pass
`), 0o644); err != nil {
		t.Fatal(err)
	}

	err := runCheck(dir, "User", "email", "")
	if err == nil {
		t.Fatal("expected error when no models detected")
	}
	if !strings.Contains(err.Error(), "no models detected") {
		t.Errorf("error should mention 'no models detected', got: %v", err)
	}
}

func TestRunCheck_ParseError(t *testing.T) {
	orig := parseModels
	t.Cleanup(func() { parseModels = orig })
	parseModels = func(_ []byte) ([]parser.Field, error) {
		return nil, fmt.Errorf("injected parse error")
	}

	dir := setupFixture(t)
	modelsFile := filepath.Join(dir, "accounts", "models.py")
	err := runCheck(dir, "User", "email", modelsFile)
	if err == nil {
		t.Fatal("expected error from parse failure")
	}
	if !strings.Contains(err.Error(), "injected parse error") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunCheck_ConflictRelPathFallback(t *testing.T) {
	orig := filepathRelFn
	t.Cleanup(func() { filepathRelFn = orig })
	filepathRelFn = func(_, path string) (string, error) {
		return "", fmt.Errorf("injected rel error")
	}

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
	if !strings.Contains(err.Error(), "multiple files") {
		t.Errorf("expected 'multiple files' error, got: %v", err)
	}
}

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shinagawa-web/colref/internal/orm"
	"github.com/shinagawa-web/colref/internal/schema"
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

func TestRunCheck_Django_ParseModelsWithSetError(t *testing.T) {
	origFn := parseModelsWithSet
	parseModelsWithSet = func(src []byte, modelSet map[string]bool) ([]schema.Field, error) {
		return nil, fmt.Errorf("injected parse error")
	}
	defer func() { parseModelsWithSet = origFn }()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "models.py"), []byte("class Foo: pass"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := runCheck(dir, "Foo", "bar", "django")
	if err == nil {
		t.Fatal("expected error from parseModelsWithSet injection")
	}
	if !strings.Contains(err.Error(), "injected parse error") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunCheck_Django_BuildModelSetError(t *testing.T) {
	origFn := buildModelSet
	buildModelSet = func(sources [][]byte) (map[string]bool, error) {
		return nil, fmt.Errorf("injected model set error")
	}
	defer func() { buildModelSet = origFn }()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "models.py"), []byte("class Foo: pass"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := runCheck(dir, "Foo", "bar", "django")
	if err == nil {
		t.Fatal("expected error from buildModelSet injection")
	}
	if !strings.Contains(err.Error(), "injected model set error") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunCheck_Django_CrossFileInheritance(t *testing.T) {
	dir := t.TempDir()

	core := filepath.Join(dir, "core")
	order := filepath.Join(dir, "order")
	if err := os.MkdirAll(core, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(order, 0o755); err != nil {
		t.Fatal(err)
	}

	// ModelWithMetadata is a Django model but its name does not end in "Model".
	if err := os.WriteFile(filepath.Join(core, "models.py"), []byte(`
from django.db import models

class ModelWithMetadata(models.Model):
    private_metadata = models.JSONField(default=dict)
    metadata = models.JSONField(default=dict)
`), 0o644); err != nil {
		t.Fatal(err)
	}

	// Order inherits from ModelWithMetadata (defined in a different file).
	if err := os.WriteFile(filepath.Join(order, "models.py"), []byte(`
from core.models import ModelWithMetadata

class Order(ModelWithMetadata):
    number = models.CharField(max_length=50)
    status = models.CharField(max_length=32)
`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(order, "views.py"), []byte(`
def show(order):
    return order.status
`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := runCheck(dir, "Order", "status", "django"); err != nil {
		t.Fatalf("cross-file inheritance: Order.status should be detected: %v", err)
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

func TestRunCheck_Django_ModelsPackage_Basic(t *testing.T) {
	dir := t.TempDir()
	modelsDir := filepath.Join(dir, "zerver", "models")
	if err := os.MkdirAll(modelsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []struct{ name, content string }{
		{"__init__.py", ""},
		{"users.py", "from django.db import models\nclass User(models.Model):\n    email = models.EmailField()\n"},
		{"messages.py", "from django.db import models\nclass Message(models.Model):\n    text = models.TextField()\n"},
	} {
		if err := os.WriteFile(filepath.Join(modelsDir, f.name), []byte(f.content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "zerver", "views.py"), []byte("def show(u): return u.email\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runCheck(dir, "User", "email", "django"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunCheck_Django_ModelsPackage_CrossFileRef(t *testing.T) {
	dir := t.TempDir()
	modelsDir := filepath.Join(dir, "blog", "models")
	if err := os.MkdirAll(modelsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(modelsDir, "users.py"), []byte("from django.db import models\nclass User(models.Model):\n    name = models.CharField(max_length=100)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(modelsDir, "posts.py"), []byte("from django.db import models\nfrom blog.models.users import User\nclass Post(User):\n    title = models.CharField(max_length=200)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "blog", "views.py"), []byte("def show(p): return p.title\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runCheck(dir, "Post", "title", "django"); err != nil {
		t.Fatalf("cross-file ref in models package: %v", err)
	}
}

func TestRunCheck_Django_ModelsPackage_MixedLayout(t *testing.T) {
	dir := t.TempDir()

	// app1 uses models.py
	app1 := filepath.Join(dir, "app1")
	if err := os.MkdirAll(app1, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app1, "models.py"), []byte("from django.db import models\nclass Product(models.Model):\n    name = models.CharField(max_length=100)\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// app2 uses models/ package
	app2Models := filepath.Join(dir, "app2", "models")
	if err := os.MkdirAll(app2Models, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app2Models, "order.py"), []byte("from django.db import models\nclass Order(models.Model):\n    total = models.DecimalField(max_digits=10, decimal_places=2)\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := runCheck(dir, "Product", "name", "django"); err != nil {
		t.Fatalf("mixed layout — Product.name: %v", err)
	}
	if err := runCheck(dir, "Order", "total", "django"); err != nil {
		t.Fatalf("mixed layout — Order.total: %v", err)
	}
}

func TestRunCheck_Django_AbstractModels_Basic(t *testing.T) {
	dir := t.TempDir()
	app := filepath.Join(dir, "catalogue")
	if err := os.MkdirAll(app, 0o755); err != nil {
		t.Fatal(err)
	}
	abstractContent := "from django.db import models\nclass AbstractProductClass(models.Model):\n    name = models.CharField(max_length=255)\n    class Meta:\n        abstract = True\n"
	modelsContent := "from catalogue.abstract_models import AbstractProductClass\nclass ProductClass(AbstractProductClass):\n    pass\n"
	viewContent := "def show(p): return p.name\n"
	for _, f := range []struct{ path, content string }{
		{filepath.Join(app, "abstract_models.py"), abstractContent},
		{filepath.Join(app, "models.py"), modelsContent},
		{filepath.Join(dir, "catalogue", "views.py"), viewContent},
	} {
		if err := os.WriteFile(f.path, []byte(f.content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Fields are declared on AbstractProductClass; users query the abstract class directly.
	if err := runCheck(dir, "AbstractProductClass", "name", "django"); err != nil {
		t.Fatalf("abstract_models.py pattern: %v", err)
	}
}

func TestRunCheck_Django_AbstractModels_OnlyAbstractFile(t *testing.T) {
	dir := t.TempDir()
	app := filepath.Join(dir, "catalogue")
	if err := os.MkdirAll(app, 0o755); err != nil {
		t.Fatal(err)
	}
	abstractContent := "from django.db import models\nclass AbstractProduct(models.Model):\n    slug = models.SlugField()\n    class Meta:\n        abstract = True\n"
	viewContent := "def show(p): return p.slug\n"
	for _, f := range []struct{ path, content string }{
		{filepath.Join(app, "abstract_models.py"), abstractContent},
		{filepath.Join(dir, "catalogue", "views.py"), viewContent},
	} {
		if err := os.WriteFile(f.path, []byte(f.content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := runCheck(dir, "AbstractProduct", "slug", "django"); err != nil {
		t.Fatalf("abstract_models.py only: %v", err)
	}
}

func TestRunCheck_Django_ModelsPackage_SkipHiddenDir(t *testing.T) {
	dir := t.TempDir()
	hiddenModels := filepath.Join(dir, ".venv", "zerver", "models")
	if err := os.MkdirAll(hiddenModels, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hiddenModels, "users.py"), []byte("from django.db import models\nclass User(models.Model):\n    email = models.EmailField()\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := runCheck(dir, "User", "email", "django")
	if err == nil {
		t.Fatal("expected error: models/ inside hidden dir should be skipped")
	}
	if !strings.Contains(err.Error(), "models.py") || !strings.Contains(err.Error(), "models/") {
		t.Errorf("error should mention both models.py and models/, got: %v", err)
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

func setupRailsMigrationsFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	migrateDir := filepath.Join(dir, "db", "migrate")
	if err := os.MkdirAll(migrateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(migrateDir, "20230101000000_create_users.rb"), []byte(`
class CreateUsers < ActiveRecord::Migration[7.0]
  def change
    create_table "users" do |t|
      t.string "email", null: false
      t.string "name"
    end
  end
end
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app.rb"), []byte("user.email\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestRunCheck_Rails_NoSchemaFile(t *testing.T) {
	// No schema.rb: fall back to db/migrate/ and reconstruct schema.
	dir := setupRailsMigrationsFixture(t)
	if err := runCheck(dir, "User", "email", "rails"); err != nil {
		t.Fatalf("migrations fallback should not error: %v", err)
	}
}

func TestRunCheck_Rails_NoSchemaFile_NoMigrateDir(t *testing.T) {
	// Neither schema.rb nor db/migrate/ present.
	err := runCheck(t.TempDir(), "User", "email", "rails")
	if err == nil {
		t.Fatal("expected error when both schema.rb and db/migrate/ are absent")
	}
	if !strings.Contains(err.Error(), "migrate") {
		t.Errorf("error should mention 'migrate', got: %v", err)
	}
}

func TestRunCheck_Rails_NoSchemaFile_ParseMigrationsError(t *testing.T) {
	orig := parseMigrations
	parseMigrations = func(dir string) ([]schema.Field, error) {
		return nil, fmt.Errorf("injected migrations error")
	}
	defer func() { parseMigrations = orig }()

	dir := t.TempDir()
	// No schema.rb triggers the migrations path.
	err := runCheck(dir, "User", "email", "rails")
	if err == nil {
		t.Fatal("expected error from parseMigrations injection")
	}
	if !strings.Contains(err.Error(), "injected migrations error") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunCheck_Rails_SchemaReadPermError(t *testing.T) {
	dir := t.TempDir()
	dbDir := filepath.Join(dir, "db")
	if err := os.MkdirAll(dbDir, 0o755); err != nil {
		t.Fatal(err)
	}
	schemaPath := filepath.Join(dbDir, "schema.rb")
	if err := os.WriteFile(schemaPath, []byte("# schema"), 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(schemaPath, 0o644) })

	err := runCheck(dir, "User", "email", "rails")
	if err == nil {
		t.Fatal("expected error for unreadable schema.rb")
	}
	if !strings.Contains(err.Error(), "read") {
		t.Errorf("expected 'read' in error, got: %v", err)
	}
}

func TestRunCheck_Rails_ParseError(t *testing.T) {
	origParse := parseSchemaRb
	parseSchemaRb = func(src []byte) ([]schema.Field, error) {
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

func setupRailsStructureFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	dbDir := filepath.Join(dir, "db")
	if err := os.MkdirAll(dbDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dbDir, "structure.sql"), []byte(`
CREATE TABLE "users" (
  "id" bigint NOT NULL,
  "email" character varying NOT NULL,
  "name" character varying
);
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app.rb"), []byte(`user.email`), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestRunCheck_Rails_StructureSql_RefsFound(t *testing.T) {
	// No schema.rb: falls through to structure.sql.
	dir := setupRailsStructureFixture(t)
	if err := runCheck(dir, "User", "email", "rails"); err != nil {
		t.Fatalf("structure.sql path should not error: %v", err)
	}
}

func TestRunCheck_Rails_StructureSql_SchemaRbWins(t *testing.T) {
	// Both schema.rb and structure.sql present: schema.rb takes priority.
	dir := setupRailsFixture(t)
	dbDir := filepath.Join(dir, "db")
	// Write a structure.sql with a different model so we can tell which was used.
	if err := os.WriteFile(filepath.Join(dbDir, "structure.sql"), []byte(`
CREATE TABLE "posts" (
  "id" bigint NOT NULL,
  "title" character varying
);
`), 0o644); err != nil {
		t.Fatal(err)
	}
	// schema.rb defines User; structure.sql defines Post.
	// If schema.rb wins, "User"/"email" should be found.
	if err := runCheck(dir, "User", "email", "rails"); err != nil {
		t.Fatalf("schema.rb should win over structure.sql: %v", err)
	}
}

func TestRunCheck_Rails_StructureSql_FallsBackToMigrations(t *testing.T) {
	// No schema.rb, no structure.sql: falls back to db/migrate/.
	dir := setupRailsMigrationsFixture(t)
	if err := runCheck(dir, "User", "email", "rails"); err != nil {
		t.Fatalf("migrations fallback should not error: %v", err)
	}
}

func TestRunCheck_Rails_StructureSql_ReadError(t *testing.T) {
	dir := t.TempDir()
	dbDir := filepath.Join(dir, "db")
	if err := os.MkdirAll(dbDir, 0o755); err != nil {
		t.Fatal(err)
	}
	sqlPath := filepath.Join(dbDir, "structure.sql")
	if err := os.WriteFile(sqlPath, []byte("-- sql"), 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(sqlPath, 0o644) })

	err := runCheck(dir, "User", "email", "rails")
	if err == nil {
		t.Fatal("expected error for unreadable structure.sql")
	}
	if !strings.Contains(err.Error(), "read") {
		t.Errorf("expected 'read' in error, got: %v", err)
	}
}

func TestRunCheck_Rails_StructureSql_ParseError(t *testing.T) {
	orig := parseStructureSql
	parseStructureSql = func(src []byte) ([]schema.Field, error) {
		return nil, fmt.Errorf("injected structure.sql parse error")
	}
	defer func() { parseStructureSql = orig }()

	dir := t.TempDir()
	dbDir := filepath.Join(dir, "db")
	if err := os.MkdirAll(dbDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dbDir, "structure.sql"), []byte("-- sql"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := runCheck(dir, "User", "email", "rails")
	if err == nil {
		t.Fatal("expected parse error")
	}
	if !strings.Contains(err.Error(), "injected structure.sql parse error") {
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

func TestValidateFormat(t *testing.T) {
	for _, ok := range []string{"text", "json"} {
		if err := validateFormat(ok); err != nil {
			t.Errorf("validateFormat(%q) = %v, want nil", ok, err)
		}
	}
	if err := validateFormat("xml"); err == nil {
		t.Error("validateFormat(\"xml\") = nil, want error")
	}
}

func TestBuildResult(t *testing.T) {
	refs := []orm.Reference{
		{File: "app/models/user.rb", Line: 12, Text: "validates :email"},
		{File: "app/mailers/user_mailer.rb", Line: 8, Text: "user.email"},
	}
	got := buildResult("User", "email", "rails", 42, refs)

	if got.Model != "User" || got.Field != "email" || got.Orm != "rails" {
		t.Errorf("identity fields wrong: %+v", got)
	}
	if got.FilesScanned != 42 {
		t.Errorf("FilesScanned = %d, want 42", got.FilesScanned)
	}
	if got.ReferenceCount != len(got.References) || got.ReferenceCount != 2 {
		t.Errorf("ReferenceCount = %d, len(References) = %d, want 2", got.ReferenceCount, len(got.References))
	}
	if got.References[0] != (checkRefOut{File: "app/models/user.rb", Line: 12, Text: "validates :email"}) {
		t.Errorf("References[0] = %+v", got.References[0])
	}
}

func TestBuildResult_EmptyReferencesEncodesAsArray(t *testing.T) {
	got := buildResult("User", "email", "django", 5, nil)
	if got.ReferenceCount != 0 {
		t.Errorf("ReferenceCount = %d, want 0", got.ReferenceCount)
	}
	if got.References == nil {
		t.Fatal("References is nil; must be a non-nil empty slice so JSON emits [] not null")
	}

	var buf bytes.Buffer
	if err := printJSON(&buf, got); err != nil {
		t.Fatalf("printJSON: %v", err)
	}
	if !strings.Contains(buf.String(), `"references": []`) {
		t.Errorf("empty references did not encode as []:\n%s", buf.String())
	}
}

func TestPrintJSON_DoesNotHTMLEscapeSnippets(t *testing.T) {
	// Source snippets contain characters json would HTML-escape by default
	// (< > &). They must survive verbatim so downstream tools see real code.
	result := buildResult("Article", "title", "rails", 1, []orm.Reference{
		{File: "app/models/article.rb", Line: 7, Text: "scope :titled, ->(t) { where(title: t) } # article&.title"},
	})
	var buf bytes.Buffer
	if err := printJSON(&buf, result); err != nil {
		t.Fatalf("printJSON: %v", err)
	}
	got := buf.String()
	// If HTML escaping were on, these would appear as -> and &.title,
	// so their verbatim presence proves SetEscapeHTML(false) took effect.
	for _, want := range []string{"->", "&.title"} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q (HTML-escaped?):\n%s", want, got)
		}
	}
}

func TestCheckCmd_JSONOutput(t *testing.T) {
	dir := setupDjangoFixture(t)

	// Flag vars persist across cobra Execute calls in the test binary; reset.
	t.Cleanup(func() { flagFormat = "text" })

	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = orig })

	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{
		"check", dir,
		"--model", "User",
		"--field", "email",
		"--orm", "django",
		"--format", "json",
	})
	execErr := rootCmd.Execute()
	_ = w.Close()
	os.Stdout = orig

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if execErr != nil {
		t.Fatalf("unexpected error: %v", execErr)
	}

	var result checkResult
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\noutput:\n%s", err, out)
	}
	if result.Model != "User" || result.Field != "email" || result.Orm != "django" {
		t.Errorf("unexpected result identity: %+v", result)
	}
	if result.ReferenceCount != len(result.References) {
		t.Errorf("ReferenceCount = %d but len(References) = %d", result.ReferenceCount, len(result.References))
	}
	if result.ReferenceCount == 0 {
		t.Error("expected at least one reference for User.email")
	}
}

package parser

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	sitter "github.com/smacker/go-tree-sitter"
)

func writeMigration(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func fieldNames(fields []Field, model string) []string {
	var names []string
	for _, f := range fields {
		if f.Model == model {
			names = append(names, f.Name)
		}
	}
	sort.Strings(names)
	return names
}

func TestParseMigrations_CreateTable(t *testing.T) {
	dir := t.TempDir()
	writeMigration(t, dir, "20230101000000_create_users.rb", `
class CreateUsers < ActiveRecord::Migration[7.0]
  def change
    create_table "users" do |t|
      t.string "email", null: false
      t.string "name"
    end
  end
end
`)
	fields, err := ParseMigrations(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := fieldNames(fields, "User")
	want := []string{"email", "name"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParseMigrations_SymbolTableName(t *testing.T) {
	dir := t.TempDir()
	writeMigration(t, dir, "20230101000000_create_users.rb", `
class CreateUsers < ActiveRecord::Migration[7.0]
  def change
    create_table :users do |t|
      t.string :email
    end
  end
end
`)
	fields, err := ParseMigrations(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := fieldNames(fields, "User")
	if len(got) != 1 || got[0] != "email" {
		t.Errorf("got %v, want [email]", got)
	}
}

func TestParseMigrations_AddColumn(t *testing.T) {
	dir := t.TempDir()
	writeMigration(t, dir, "20230101000000_create_users.rb", `
create_table :users do |t|
  t.string :email
end
`)
	writeMigration(t, dir, "20230201000000_add_name_to_users.rb", `
add_column :users, :name, :string
`)
	fields, err := ParseMigrations(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := fieldNames(fields, "User")
	want := []string{"email", "name"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParseMigrations_RemoveColumn(t *testing.T) {
	dir := t.TempDir()
	writeMigration(t, dir, "20230101000000_create_users.rb", `
create_table :users do |t|
  t.string :email
  t.string :legacy
end
`)
	writeMigration(t, dir, "20230201000000_remove_legacy.rb", `
remove_column :users, :legacy
`)
	fields, err := ParseMigrations(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := fieldNames(fields, "User")
	if len(got) != 1 || got[0] != "email" {
		t.Errorf("got %v, want [email]", got)
	}
}

func TestParseMigrations_RenameColumn(t *testing.T) {
	dir := t.TempDir()
	writeMigration(t, dir, "20230101000000_create_users.rb", `
create_table :users do |t|
  t.string :mail
end
`)
	writeMigration(t, dir, "20230201000000_rename_mail.rb", `
rename_column :users, :mail, :email
`)
	fields, err := ParseMigrations(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := fieldNames(fields, "User")
	if len(got) != 1 || got[0] != "email" {
		t.Errorf("got %v, want [email]", got)
	}
}

func TestParseMigrations_DropTable(t *testing.T) {
	dir := t.TempDir()
	writeMigration(t, dir, "20230101000000_create_users.rb", `
create_table :users do |t|
  t.string :email
end
`)
	writeMigration(t, dir, "20230201000000_drop_users.rb", `
drop_table :users
`)
	fields, err := ParseMigrations(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := fieldNames(fields, "User")
	if len(got) != 0 {
		t.Errorf("expected no fields after drop_table, got %v", got)
	}
}

func TestParseMigrations_MultipleModels(t *testing.T) {
	dir := t.TempDir()
	writeMigration(t, dir, "20230101000000_create_tables.rb", `
create_table :users do |t|
  t.string :email
end
create_table :posts do |t|
  t.string :title
end
`)
	fields, err := ParseMigrations(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if names := fieldNames(fields, "User"); len(names) != 1 || names[0] != "email" {
		t.Errorf("User: got %v", names)
	}
	if names := fieldNames(fields, "Post"); len(names) != 1 || names[0] != "title" {
		t.Errorf("Post: got %v", names)
	}
}

func TestParseMigrations_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	fields, err := ParseMigrations(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fields) != 0 {
		t.Errorf("expected empty fields, got %v", fields)
	}
}

func TestParseMigrations_NonExistentDir(t *testing.T) {
	_, err := ParseMigrations("/nonexistent/dir/that/does/not/exist")
	if err == nil {
		t.Fatal("expected error for non-existent directory")
	}
}

func TestParseMigrations_UnreadableFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "20230101000000_create_users.rb")
	if err := os.WriteFile(path, []byte("create_table :users do |t| end"), 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })

	_, err := ParseMigrations(dir)
	if err == nil {
		t.Fatal("expected error for unreadable migration file")
	}
}

func TestParseMigrations_ParseError(t *testing.T) {
	orig := parseCtxFnRuby
	parseCtxFnRuby = func(p *sitter.Parser, ctx context.Context, oldTree *sitter.Tree, src []byte) (*sitter.Tree, error) {
		return nil, fmt.Errorf("injected parse error")
	}
	defer func() { parseCtxFnRuby = orig }()

	dir := t.TempDir()
	writeMigration(t, dir, "20230101000000_create_users.rb", "create_table :users do |t| end")

	_, err := ParseMigrations(dir)
	if err == nil {
		t.Fatal("expected error from parse injection")
	}
}

func TestParseMigrations_IgnoresNonRbFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# migrations"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeMigration(t, dir, "20230101000000_create_users.rb", `
create_table :users do |t|
  t.string :email
end
`)
	fields, err := ParseMigrations(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fields) == 0 {
		t.Error("expected fields from .rb file")
	}
}

func TestParseMigrations_AddColumnMissingArgs(t *testing.T) {
	dir := t.TempDir()
	// add_column with only table (no col) — should be silently skipped.
	writeMigration(t, dir, "20230101000000_noop.rb", `
add_column :users
`)
	fields, err := ParseMigrations(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fields) != 0 {
		t.Errorf("expected no fields, got %v", fields)
	}
}

func TestParseMigrations_RemoveColumnMissingArgs(t *testing.T) {
	dir := t.TempDir()
	writeMigration(t, dir, "20230101000000_noop.rb", `
remove_column :users
`)
	fields, err := ParseMigrations(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fields) != 0 {
		t.Errorf("expected no fields, got %v", fields)
	}
}

func TestParseMigrations_RenameColumnMissingArgs(t *testing.T) {
	dir := t.TempDir()
	// rename_column with only two args — should be silently skipped.
	writeMigration(t, dir, "20230101000000_noop.rb", `
rename_column :users, :old_name
`)
	fields, err := ParseMigrations(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fields) != 0 {
		t.Errorf("expected no fields, got %v", fields)
	}
}

func TestParseMigrations_RemoveColumnTableNotInSchema(t *testing.T) {
	dir := t.TempDir()
	// remove_column on a table that was never created — should be silently skipped.
	writeMigration(t, dir, "20230101000000_noop.rb", `
remove_column :ghost, :col
`)
	fields, err := ParseMigrations(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fields) != 0 {
		t.Errorf("expected no fields, got %v", fields)
	}
}

func TestParseMigrations_RenameColumnTableNotInSchema(t *testing.T) {
	dir := t.TempDir()
	writeMigration(t, dir, "20230101000000_noop.rb", `
rename_column :ghost, :old, :new_name
`)
	fields, err := ParseMigrations(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fields) != 0 {
		t.Errorf("expected no fields, got %v", fields)
	}
}

func TestParseMigrations_CreateTableNoDoBlock(t *testing.T) {
	dir := t.TempDir()
	// create_table without a block — table registered but no columns.
	writeMigration(t, dir, "20230101000000_create.rb", `
create_table "widgets", force: true
`)
	fields, err := ParseMigrations(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := fieldNames(fields, "Widget")
	if len(got) != 0 {
		t.Errorf("expected no columns, got %v", got)
	}
}

// TestParseMigrations_NilArgBranches covers defensive nil-arg guards in all
// operation handlers by passing block-only calls (no argument list).
func TestParseMigrations_NilArgBranches(t *testing.T) {
	dir := t.TempDir()
	// Each call has a block but no argument list → args == nil in all handlers.
	writeMigration(t, dir, "20230101000000_noop.rb", `
create_table do |t| end
add_column do end
remove_column do end
rename_column do end
drop_table do end
`)
	fields, err := ParseMigrations(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fields) != 0 {
		t.Errorf("expected no fields, got %v", fields)
	}
}

func TestParseMigrations_CreateTableInvalidFirstArg(t *testing.T) {
	dir := t.TempDir()
	// Integer as first arg → migArg returns "" → table == "" branch.
	writeMigration(t, dir, "20230101000000_create.rb", `
create_table 42 do |t|
  t.string :email
end
`)
	fields, err := ParseMigrations(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fields) != 0 {
		t.Errorf("expected no fields for invalid table name, got %v", fields)
	}
}

func TestParseMigrations_CreateTableEmptyBlock(t *testing.T) {
	dir := t.TempDir()
	// Empty block → body_statement may be absent → body == nil branch.
	writeMigration(t, dir, "20230101000000_create.rb", `
create_table :empties do
end
`)
	fields, err := ParseMigrations(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := fieldNames(fields, "Empty")
	if len(got) != 0 {
		t.Errorf("expected no columns, got %v", got)
	}
}

func TestParseMigrations_CreateTableNonCallInBlock(t *testing.T) {
	dir := t.TempDir()
	// Assignment in block → stmt.Type() != "call" branch.
	writeMigration(t, dir, "20230101000000_create.rb", `
create_table :users do |t|
  x = 1
  t.string :email
end
`)
	fields, err := ParseMigrations(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := fieldNames(fields, "User")
	if len(got) != 1 || got[0] != "email" {
		t.Errorf("got %v, want [email]", got)
	}
}

func TestParseMigrations_CreateTableTimestamps(t *testing.T) {
	dir := t.TempDir()
	// t.timestamps has no string/symbol args → colArgs present but col == "".
	// t.timestamps may also have no argument list at all → colArgs == nil.
	writeMigration(t, dir, "20230101000000_create.rb", `
create_table :users do |t|
  t.timestamps
  t.string :email
end
`)
	fields, err := ParseMigrations(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := fieldNames(fields, "User")
	if len(got) != 1 || got[0] != "email" {
		t.Errorf("got %v, want [email]", got)
	}
}

func TestParseMigrations_CreateTableInvalidColName(t *testing.T) {
	dir := t.TempDir()
	// Integer as column name → migArg returns "" → col == "" branch.
	writeMigration(t, dir, "20230101000000_create.rb", `
create_table :users do |t|
  t.string 42
  t.string :email
end
`)
	fields, err := ParseMigrations(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := fieldNames(fields, "User")
	if len(got) != 1 || got[0] != "email" {
		t.Errorf("got %v, want [email]", got)
	}
}

func TestParseMigrations_AddColumnNewTable(t *testing.T) {
	dir := t.TempDir()
	// add_column for a table never seen in create_table → schema[table] == nil.
	writeMigration(t, dir, "20230101000000_add.rb", `
add_column :brand_new, :col, :string
`)
	fields, err := ParseMigrations(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := fieldNames(fields, "BrandNew")
	if len(got) != 1 || got[0] != "col" {
		t.Errorf("got %v, want [col]", got)
	}
}

func TestParseMigrations_DropTableInvalidArg(t *testing.T) {
	dir := t.TempDir()
	// Integer as table name → migArg returns "" → table == "" branch.
	writeMigration(t, dir, "20230101000000_drop.rb", `
drop_table 42
`)
	fields, err := ParseMigrations(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fields) != 0 {
		t.Errorf("expected no fields, got %v", fields)
	}
}

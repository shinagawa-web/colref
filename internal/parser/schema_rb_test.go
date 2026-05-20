package parser

import (
	"context"
	"fmt"
	"testing"

	sitter "github.com/smacker/go-tree-sitter"
)

func TestRailsParser_ParseSchema(t *testing.T) {
	src := []byte(`
ActiveRecord::Schema.define do
  create_table "users", force: :cascade do |t|
    t.string "email"
  end
end
`)
	fields, err := RailsParser{}.ParseSchema(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fields) != 1 || fields[0].Name != "email" {
		t.Errorf("want email field, got %v", fields)
	}
}

func TestParseSchemaRb_ParseError(t *testing.T) {
	orig := parseCtxFnRuby
	t.Cleanup(func() { parseCtxFnRuby = orig })
	parseCtxFnRuby = func(_ *sitter.Parser, _ context.Context, _ *sitter.Tree, _ []byte) (*sitter.Tree, error) {
		return nil, fmt.Errorf("injected parse error")
	}

	_, err := ParseSchemaRb([]byte(`ActiveRecord::Schema.define do; end`))
	if err == nil {
		t.Fatal("expected error from injected parse failure")
	}
}

func TestParseSchemaRb(t *testing.T) {
	src := []byte(`
ActiveRecord::Schema[7.0].define(version: 2024_01_15_000000) do
  create_table "users", force: :cascade do |t|
    t.string "email", null: false
    t.string "name", limit: 100
    t.integer "role", default: 0
    t.timestamps
  end

  create_table "posts", force: :cascade do |t|
    t.string "title", null: false
    t.text "body"
    t.bigint "user_id", null: false
    t.index ["user_id"], name: "index_posts_on_user_id"
  end
end
`)

	fields, err := ParseSchemaRb(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []Field{
		{Model: "User", Name: "email"},
		{Model: "User", Name: "name"},
		{Model: "User", Name: "role"},
		{Model: "Post", Name: "title"},
		{Model: "Post", Name: "body"},
		{Model: "Post", Name: "user_id"},
	}

	if len(fields) != len(want) {
		t.Fatalf("got %d fields, want %d\ngot: %v", len(fields), len(want), fields)
	}
	for i, f := range fields {
		if f != want[i] {
			t.Errorf("fields[%d] = %v, want %v", i, f, want[i])
		}
	}
}

func TestParseSchemaRb_IndexSkipped(t *testing.T) {
	// t.index has an array as first arg, not a string — should be skipped.
	src := []byte(`
ActiveRecord::Schema.define do
  create_table "users", force: :cascade do |t|
    t.string "email"
    t.index ["email"], name: "index_users_on_email", unique: true
  end
end
`)

	fields, err := ParseSchemaRb(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, f := range fields {
		if f.Name == "email" && f.Model == "User" {
			continue
		}
		if f.Name != "email" {
			t.Errorf("unexpected field %v — index should be skipped", f)
		}
	}
	if len(fields) != 1 || fields[0].Name != "email" {
		t.Errorf("want only email, got %v", fields)
	}
}

func TestParseSchemaRb_TimestampsSkipped(t *testing.T) {
	// t.timestamps has no string arg — should be skipped (v0.1 limitation).
	src := []byte(`
ActiveRecord::Schema.define do
  create_table "posts", force: :cascade do |t|
    t.string "title"
    t.timestamps
  end
end
`)

	fields, err := ParseSchemaRb(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fields) != 1 || fields[0].Name != "title" {
		t.Errorf("want only title, got %v", fields)
	}
}

func TestParseSchemaRb_NoArgsCreateTable(t *testing.T) {
	// create_table without an argument_list should be silently skipped.
	src := []byte(`
ActiveRecord::Schema.define do
  create_table do |t|
    t.string "name"
  end
end
`)

	fields, err := ParseSchemaRb(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fields) != 0 {
		t.Errorf("expected no fields for create_table without table name, got %v", fields)
	}
}

func TestParseSchemaRb_NonStringTableName(t *testing.T) {
	// create_table with a non-string first arg should be silently skipped.
	src := []byte(`
ActiveRecord::Schema.define do
  create_table :users, force: :cascade do |t|
    t.string "email"
  end
end
`)

	fields, err := ParseSchemaRb(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fields) != 0 {
		t.Errorf("expected no fields for symbol table name, got %v", fields)
	}
}

func TestParseSchemaRb_NoDoBlock(t *testing.T) {
	// create_table without a do block should be silently skipped.
	src := []byte(`
ActiveRecord::Schema.define do
  create_table "users", force: :cascade
end
`)

	fields, err := ParseSchemaRb(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fields) != 0 {
		t.Errorf("expected no fields when no do block, got %v", fields)
	}
}

func TestParseSchemaRb_EmptyDoBlock(t *testing.T) {
	// do block with no body_statement → body == nil → silently skipped.
	src := []byte(`
ActiveRecord::Schema.define do
  create_table "users" do end
end
`)

	fields, err := ParseSchemaRb(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fields) != 0 {
		t.Errorf("expected no fields for empty do block, got %v", fields)
	}
}

func TestParseSchemaRb_NonCallBodyStatement(t *testing.T) {
	// A bare string literal inside the do block produces a non-call statement.
	// It should be skipped while other column calls are still extracted.
	src := []byte("ActiveRecord::Schema.define do\n  create_table \"users\" do |t|\n    \"bare string\"\n    t.string \"email\"\n  end\nend\n")

	fields, err := ParseSchemaRb(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fields) != 1 || fields[0].Name != "email" {
		t.Errorf("want only email, got %v", fields)
	}
}

func TestTableToModel(t *testing.T) {
	cases := []struct {
		table string
		model string
	}{
		{"users", "User"},
		{"posts", "Post"},
		{"categories", "Category"},
		{"addresses", "Address"},
		{"order_items", "OrderItem"},
		{"admin_users", "AdminUser"},
		{"data", "Data"},
		{"statuses", "Status"},
		{"scheduled_statuses", "ScheduledStatus"},
		{"custom_filter_statuses", "CustomFilterStatus"},
		{"boxes", "Box"},
		{"churches", "Church"},
		{"dishes", "Dish"},
	}
	for _, c := range cases {
		got := tableToModel(c.table)
		if got != c.model {
			t.Errorf("tableToModel(%q) = %q, want %q", c.table, got, c.model)
		}
	}
}

func TestSingularize(t *testing.T) {
	cases := []struct {
		in  string
		out string
	}{
		{"users", "user"},
		{"posts", "post"},
		{"categories", "category"},
		{"addresses", "address"},
		{"items", "item"},
		{"data", "data"},
		{"s", "s"},
		{"ies", "ie"},
		// -ses/-xes/-zes/-ches/-shes → strip -es
		{"statuses", "status"},
		{"boxes", "box"},
		{"buzzes", "buzz"},
		{"churches", "church"},
		{"dishes", "dish"},
	}
	for _, c := range cases {
		got := singularize(c.in)
		if got != c.out {
			t.Errorf("singularize(%q) = %q, want %q", c.in, got, c.out)
		}
	}
}

func TestCapitalize(t *testing.T) {
	if got := capitalize(""); got != "" {
		t.Errorf("capitalize(%q) = %q, want %q", "", got, "")
	}
	if got := capitalize("user"); got != "User" {
		t.Errorf("capitalize(%q) = %q, want %q", "user", got, "User")
	}
}

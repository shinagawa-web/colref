package schema

import (
	"slices"
	"testing"
)

func TestParseStructureSql_PostgreSQL(t *testing.T) {
	src := []byte(`
SET statement_timeout = 0;

CREATE TABLE public.users (
    id bigint NOT NULL,
    email character varying NOT NULL,
    name character varying,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);

CREATE TABLE public.articles (
    id bigint NOT NULL,
    title character varying,
    body text,
    user_id bigint NOT NULL
);
`)
	fields, err := ParseStructureSql(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	userFields := fieldNames(fields, "User")
	if !slices.Equal(userFields, []string{"created_at", "email", "id", "name", "updated_at"}) {
		t.Errorf("User fields: got %v", userFields)
	}
	articleFields := fieldNames(fields, "Article")
	if !slices.Equal(articleFields, []string{"body", "id", "title", "user_id"}) {
		t.Errorf("Article fields: got %v", articleFields)
	}
}

func TestParseStructureSql_SQLite(t *testing.T) {
	src := []byte(`
CREATE TABLE "users" (
  "id" integer NOT NULL PRIMARY KEY AUTOINCREMENT,
  "email" varchar(255) NOT NULL,
  "name" varchar(100) DEFAULT NULL,
  "created_at" datetime(6) NOT NULL,
  "updated_at" datetime(6) NOT NULL
);
`)
	fields, err := ParseStructureSql(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := fieldNames(fields, "User")
	if !slices.Equal(got, []string{"created_at", "email", "id", "name", "updated_at"}) {
		t.Errorf("got %v", got)
	}
}

func TestParseStructureSql_MySQL(t *testing.T) {
	src := []byte("CREATE TABLE `articles` (\n" +
		"  `id` bigint NOT NULL AUTO_INCREMENT,\n" +
		"  `title` varchar(255) DEFAULT NULL,\n" +
		"  `body` longtext,\n" +
		"  PRIMARY KEY (`id`),\n" +
		"  KEY `index_articles_on_title` (`title`)\n" +
		") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;\n")
	fields, err := ParseStructureSql(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := fieldNames(fields, "Article")
	if !slices.Equal(got, []string{"body", "id", "title"}) {
		t.Errorf("got %v", got)
	}
}

func TestParseStructureSql_ConstraintsSkipped(t *testing.T) {
	src := []byte(`
CREATE TABLE "articles" (
  "id" bigint NOT NULL,
  "title" character varying,
  PRIMARY KEY ("id"),
  UNIQUE ("title"),
  CONSTRAINT "articles_title_check" CHECK (char_length(title) > 0),
  FOREIGN KEY ("user_id") REFERENCES "users" ("id")
);
`)
	fields, err := ParseStructureSql(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := fieldNames(fields, "Article")
	if !slices.Equal(got, []string{"id", "title"}) {
		t.Errorf("constraints should be skipped, got %v", got)
	}
}

func TestParseStructureSql_AlterTableAddColumn(t *testing.T) {
	src := []byte(`
CREATE TABLE "users" (
  "id" bigint NOT NULL,
  "email" character varying NOT NULL
);

ALTER TABLE "users" ADD COLUMN "bio" text;
ALTER TABLE ONLY "users" ADD COLUMN "avatar_url" character varying;
`)
	fields, err := ParseStructureSql(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := fieldNames(fields, "User")
	if !slices.Equal(got, []string{"avatar_url", "bio", "email", "id"}) {
		t.Errorf("got %v", got)
	}
}

func TestParseStructureSql_AlterTableAddColumn_SchemaPrefix(t *testing.T) {
	src := []byte(`
ALTER TABLE public.users ADD COLUMN bio text;
`)
	fields, err := ParseStructureSql(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := fieldNames(fields, "User")
	if !slices.Equal(got, []string{"bio"}) {
		t.Errorf("got %v", got)
	}
}

func TestParseStructureSql_AlterTableAddConstraint_Skipped(t *testing.T) {
	// ADD CONSTRAINT should not be confused with ADD COLUMN.
	src := []byte(`
CREATE TABLE "users" (
  "id" bigint NOT NULL,
  "email" character varying NOT NULL
);

ALTER TABLE ONLY "users" ADD CONSTRAINT "users_pkey" PRIMARY KEY ("id");
`)
	fields, err := ParseStructureSql(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := fieldNames(fields, "User")
	if !slices.Equal(got, []string{"email", "id"}) {
		t.Errorf("constraint ALTER should be skipped, got %v", got)
	}
}

func TestParseStructureSql_SchemaRbWins(t *testing.T) {
	// Verifies the detection order at the parser level (both parse correctly).
	// Integration-level ordering is covered in check_test.go.
	rbSrc := []byte(`
ActiveRecord::Schema.define do
  create_table "users", force: :cascade do |t|
    t.string "schema_rb_field"
  end
end
`)
	sqlSrc := []byte(`
CREATE TABLE "users" (
  "id" bigint NOT NULL,
  "structure_sql_field" character varying
);
`)
	rbFields, err := ParseSchemaRb(rbSrc)
	if err != nil {
		t.Fatal(err)
	}
	sqlFields, err := ParseStructureSql(sqlSrc)
	if err != nil {
		t.Fatal(err)
	}

	if fieldNames(rbFields, "User")[0] != "schema_rb_field" {
		t.Errorf("schema.rb parsed incorrectly: %v", rbFields)
	}
	if fieldNames(sqlFields, "User")[0] != "id" {
		t.Errorf("structure.sql parsed incorrectly: %v", sqlFields)
	}
}

func TestParseStructureSql_IfNotExists(t *testing.T) {
	src := []byte(`
CREATE TABLE IF NOT EXISTS "users" (
  "id" bigint NOT NULL,
  "email" character varying NOT NULL
);
`)
	fields, err := ParseStructureSql(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := fieldNames(fields, "User")
	if !slices.Equal(got, []string{"email", "id"}) {
		t.Errorf("got %v", got)
	}
}

func TestParseStructureSql_Empty(t *testing.T) {
	fields, err := ParseStructureSql([]byte(""))
	if err != nil {
		t.Fatal(err)
	}
	if len(fields) != 0 {
		t.Errorf("want no fields from empty input, got %v", fields)
	}
}

func TestParseStructureSql_UnquotedNames(t *testing.T) {
	src := []byte(`
CREATE TABLE users (
    id bigint NOT NULL,
    email varchar(255) NOT NULL
);
`)
	fields, err := ParseStructureSql(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := fieldNames(fields, "User")
	if !slices.Equal(got, []string{"email", "id"}) {
		t.Errorf("got %v", got)
	}
}

func TestParseStructureSql_AlterAddColumn_IfNotExists(t *testing.T) {
	src := []byte(`
CREATE TABLE "users" (
  "id" bigint NOT NULL
);

ALTER TABLE "users" ADD COLUMN IF NOT EXISTS "bio" text;
`)
	fields, err := ParseStructureSql(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := fieldNames(fields, "User")
	if !slices.Equal(got, []string{"bio", "id"}) {
		t.Errorf("got %v", got)
	}
}

// Tests for internal helper functions (package-level access).

func TestSqlNextIdent(t *testing.T) {
	tests := []struct {
		input     string
		wantIdent string
		wantRest  string
	}{
		{`"users" more`, "users", " more"},
		{"`users` more", "users", " more"},
		{"users more", "users", " more"},
		{"users", "users", ""},          // unquoted with no separator → rest is ""
		{`"unterminated`, "", ""},       // unterminated double-quote
		{"`unterminated", "", ""},       // unterminated backtick
		{"", "", ""},
	}
	for _, tt := range tests {
		ident, rest := sqlNextIdent(tt.input)
		if ident != tt.wantIdent || rest != tt.wantRest {
			t.Errorf("sqlNextIdent(%q) = (%q, %q), want (%q, %q)",
				tt.input, ident, rest, tt.wantIdent, tt.wantRest)
		}
	}
}

func TestSqlExtractCreateTableName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"not a create statement", ""},
		{"CREATE TABLE ", ""},                           // empty after keyword → ident1 == ""
		{`CREATE TABLE "users" (`, "users"},
		{"CREATE TABLE public.users (", "users"},
		{`CREATE TABLE "public"."users" (`, "users"},
		{`CREATE TABLE public.( (`, "public"},           // dot then non-ident → fall back to ident1
	}
	for _, tt := range tests {
		if got := sqlExtractCreateTableName(tt.input); got != tt.want {
			t.Errorf("sqlExtractCreateTableName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSqlExtractAlterAddColumn(t *testing.T) {
	tests := []struct {
		input     string
		wantTable string
		wantCol   string
	}{
		{"RENAME TABLE users TO new_name", "", ""},                             // no "ALTER TABLE" present
		{"ALTER TABLE ", "", ""},                                                 // ident1 == ""
		{`ALTER TABLE "users" RENAME TO new_users`, "", ""},                     // addIdx < 0
		{`ALTER TABLE "users" ADDING COLUMN "bio" text`, "", ""},               // "ADD" is part of "ADDING"
		{`ALTER TABLE "users" ADD COLUMN "bio" text`, "users", "bio"},
		{`ALTER TABLE "users" ADD COLUMN IF NOT EXISTS "bio" text`, "users", "bio"},
		{`ALTER TABLE public.( ADD COLUMN "email" varchar`, "public", "email"},  // schema. fallback to ident1
	}
	for _, tt := range tests {
		table, col := sqlExtractAlterAddColumn(tt.input)
		if table != tt.wantTable || col != tt.wantCol {
			t.Errorf("sqlExtractAlterAddColumn(%q) = (%q, %q), want (%q, %q)",
				tt.input, table, col, tt.wantTable, tt.wantCol)
		}
	}
}

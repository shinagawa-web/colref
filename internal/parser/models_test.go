package parser

import (
	"context"
	"testing"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/python"
)

func TestParseModels(t *testing.T) {
	src := []byte(`
from django.db import models

class User(models.Model):
    email = models.EmailField(max_length=254)
    name = models.CharField(max_length=100)
    created_at = models.DateTimeField(auto_now_add=True)

    class Meta:
        db_table = "users"

    def __str__(self):
        return self.email

class Post(models.Model):
    title = models.CharField(max_length=200)
    body = models.TextField()
    author = models.ForeignKey(User, on_delete=models.CASCADE)

class NotAModel:
    value = models.CharField()
`)

	fields, err := ParseModels(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []Field{
		{Model: "User", Name: "email"},
		{Model: "User", Name: "name"},
		{Model: "User", Name: "created_at"},
		{Model: "Post", Name: "title"},
		{Model: "Post", Name: "body"},
		{Model: "Post", Name: "author"},
	}

	if len(fields) != len(want) {
		t.Fatalf("got %d fields, want %d\n got: %v", len(fields), len(want), fields)
	}
	for i, f := range fields {
		if f != want[i] {
			t.Errorf("fields[%d] = %v, want %v", i, f, want[i])
		}
	}
}

func TestParseModels_AbstractBase(t *testing.T) {
	src := []byte(`
from django.db import models

class TimestampedModel(models.Model):
    created_at = models.DateTimeField(auto_now_add=True)

    class Meta:
        abstract = True

class Article(TimestampedModel):
    title = models.CharField(max_length=200)
`)

	fields, err := ParseModels(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(fields) == 0 {
		t.Fatal("expected fields, got none")
	}

	// TimestampedModel ends in "Model" → included
	// Article inherits TimestampedModel which ends in "Model" → included
	models := map[string]bool{}
	for _, f := range fields {
		models[f.Model] = true
	}
	if !models["TimestampedModel"] {
		t.Error("expected TimestampedModel to be included")
	}
	if !models["Article"] {
		t.Error("expected Article to be included")
	}
}

// TestParseModels_ParseCtxError covers the ParseCtx error path by injecting
// a failing parseCtxFn.
func TestParseModels_ParseCtxError(t *testing.T) {
	orig := parseCtxFn
	parseCtxFn = func(p *sitter.Parser, ctx context.Context, oldTree *sitter.Tree, src []byte) (*sitter.Tree, error) {
		return nil, ctx.Err()
	}
	defer func() { parseCtxFn = orig }()

	// Use a cancelled context to return a non-nil error from the injected fn.
	// The injected fn returns ctx.Err() but ParseModels uses context.Background(),
	// so we simulate the error by returning a sentinel error regardless.
	parseCtxFn = func(p *sitter.Parser, ctx context.Context, oldTree *sitter.Tree, src []byte) (*sitter.Tree, error) {
		return nil, context.Canceled
	}

	_, err := ParseModels([]byte(`x = 1`))
	if err == nil {
		t.Fatal("expected error from ParseModels when parseCtxFn fails")
	}
}

// TestExtractFields_NoBody covers the body==nil branch in extractFields by
// passing a non-class node that has no "body" field.
func TestExtractFields_NoBody(t *testing.T) {
	src := []byte(`x = 1`)
	p := sitter.NewParser()
	p.SetLanguage(python.GetLanguage())
	tree, err := p.ParseCtx(context.Background(), nil, src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The root child is an expression_statement, which has no "body" field.
	stmt := tree.RootNode().Child(0)
	if got := extractFields(stmt, src); got != nil {
		t.Errorf("expected nil for non-class node, got %v", got)
	}
}

// TestParseModels_NoModel covers the isModelClass false-return branch:
// a class that does not inherit from anything ending in "Model".
func TestParseModels_NoModel(t *testing.T) {
	src := []byte(`
class Helper(SomeBase):
    value = 42
`)
	fields, err := ParseModels(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fields) != 0 {
		t.Errorf("expected 0 fields for non-model class, got %d: %v", len(fields), fields)
	}
}

// TestParseModels_SkipNonAssignments covers the extractFields branches that
// skip non-assignment statements and non-identifier left-hand sides.
func TestParseModels_SkipNonAssignments(t *testing.T) {
	src := []byte(`
from django.db import models

class MyModel(models.Model):
    email = models.EmailField()
    some_call()
    self.attr = models.CharField()
`)
	fields, err := ParseModels(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Only 'email' should be detected (simple identifier lhs + django field rhs).
	// 'some_call()' is an expression_statement but not an assignment.
	// 'self.attr' has attribute lhs (not identifier) — skipped.
	found := false
	for _, f := range fields {
		if f.Name == "email" {
			found = true
		}
		if f.Name == "attr" {
			t.Error("'attr' should not be found (non-identifier lhs)")
		}
	}
	if !found {
		t.Error("expected 'email' field to be found")
	}
}

// TestIsDjangoField_Nil covers the nil-node early return in isDjangoField.
func TestIsDjangoField_Nil(t *testing.T) {
	if isDjangoField(nil, nil) {
		t.Error("isDjangoField(nil, nil) should return false")
	}
}

// TestIsDjangoField_BareIdentifier covers the identifier branch (lines 127-130).
// A bare identifier ending in "Field" should return true; otherwise false.
func TestIsDjangoField_BareIdentifier(t *testing.T) {
	// CharField imported directly: rhs is a bare call to CharField → identifier after unwrap.
	// tag = bare_id: identifier that does not end in "Field" → false.
	src := []byte(`
from django.db.models import CharField, IntegerField

class MyModel(models.Model):
    name = CharField(max_length=100)
    count = IntegerField()
    tag = bare_id
`)
	fields, err := ParseModels(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := map[string]bool{}
	for _, f := range fields {
		found[f.Name] = true
	}
	if !found["name"] {
		t.Error("expected 'name' field (CharField bare identifier) to be found")
	}
	if !found["count"] {
		t.Error("expected 'count' field (IntegerField bare identifier) to be found")
	}
	if found["tag"] {
		t.Error("expected 'tag' (bare_id) NOT to be found as a Django field")
	}
}

// TestIsDjangoField_NonFieldLiteral covers the final return false in isDjangoField
// when the rhs is a literal (integer or string) — not a call, attribute, or identifier.
func TestIsDjangoField_NonFieldLiteral(t *testing.T) {
	src := []byte(`
from django.db import models

class MyModel(models.Model):
    email = models.EmailField()
    MAX_LENGTH = 100
    STATUS = "active"
`)
	fields, err := ParseModels(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := map[string]bool{}
	for _, f := range fields {
		found[f.Name] = true
	}
	if !found["email"] {
		t.Error("expected 'email' to be found")
	}
	if found["MAX_LENGTH"] {
		t.Error("MAX_LENGTH (integer literal) should not be a field")
	}
	if found["STATUS"] {
		t.Error("STATUS (string literal) should not be a field")
	}
}

func BenchmarkParseModels(b *testing.B) {
	src := []byte(`
from django.db import models

class User(models.Model):
    email = models.EmailField(max_length=254)
    name = models.CharField(max_length=100)
    created_at = models.DateTimeField(auto_now_add=True)
    is_active = models.BooleanField(default=True)

class Post(models.Model):
    title = models.CharField(max_length=200)
    body = models.TextField()
    author = models.ForeignKey(User, on_delete=models.CASCADE)
    published_at = models.DateTimeField(null=True)
    slug = models.SlugField(unique=True)

class Comment(models.Model):
    post = models.ForeignKey(Post, on_delete=models.CASCADE)
    author = models.ForeignKey(User, on_delete=models.CASCADE)
    body = models.TextField()
    created_at = models.DateTimeField(auto_now_add=True)
`)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := ParseModels(src); err != nil {
			b.Fatal(err)
		}
	}
}

// TestIsDjangoField_ThirdPartyAttribute covers the attribute branch where the
// object is not "models" but the attribute name ends in "Field" (third-party fields),
// and the case where neither condition matches (returns false).
func TestIsDjangoField_ThirdPartyAttribute(t *testing.T) {
	src := []byte(`
from django.db import models

class MyModel(models.Model):
    photo = stdimage.StdImageField()
    config = SomeUtils.VALUE
`)
	fields, err := ParseModels(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := map[string]bool{}
	for _, f := range fields {
		found[f.Name] = true
	}
	// photo: attribute obj="stdimage" (not "models"), attr="StdImageField" (ends in "Field") → true
	if !found["photo"] {
		t.Error("expected 'photo' (third-party StdImageField) to be found")
	}
	// config: attribute obj="SomeUtils" (not "models"), attr="VALUE" (no "Field" suffix) → false
	if found["config"] {
		t.Error("expected 'config' (SomeUtils.VALUE) NOT to be found as a Django field")
	}
}

func TestDjangoParser_ParseSchema(t *testing.T) {
	src := []byte(`
from django.db import models

class User(models.Model):
    email = models.EmailField()
`)
	fields, err := DjangoParser{}.ParseSchema(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fields) != 1 || fields[0].Name != "email" {
		t.Errorf("unexpected fields: %v", fields)
	}
}

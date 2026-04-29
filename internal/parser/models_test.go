package parser

import (
	"context"
	"fmt"
	"testing"

	sitter "github.com/smacker/go-tree-sitter"
)

func TestParseModels_ParseError(t *testing.T) {
	orig := parseCtxFn
	t.Cleanup(func() { parseCtxFn = orig })
	parseCtxFn = func(_ *sitter.Parser, _ context.Context, _ *sitter.Tree, _ []byte) (*sitter.Tree, error) {
		return nil, fmt.Errorf("injected parse error")
	}

	_, err := ParseModels([]byte(`class User(models.Model): pass`))
	if err == nil {
		t.Fatal("expected error from injected parse failure")
	}
}

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

func TestParseModels_MethodsAndPropertiesNotExtracted(t *testing.T) {
	src := []byte(`
from django.db import models

class User(models.Model):
    email = models.EmailField()

    @property
    def display_name(self):
        return self.email

    def save(self, *args, **kwargs):
        self.email = self.email.lower()
        super().save(*args, **kwargs)
`)

	fields, err := ParseModels(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, f := range fields {
		if f.Name == "display_name" || f.Name == "save" {
			t.Errorf("method/property %q should not be extracted as a field", f.Name)
		}
	}
	if len(fields) != 1 || fields[0].Name != "email" {
		t.Errorf("want only email field, got %v", fields)
	}
}

func TestParseModels_ClassVariablesNotFields(t *testing.T) {
	src := []byte(`
from django.db import models

class User(models.Model):
    email = models.EmailField()
    MAX_LENGTH = 254
    DEFAULT_ROLE = "member"
    ACTIVE = True
`)

	fields, err := ParseModels(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, f := range fields {
		if f.Name == "MAX_LENGTH" || f.Name == "DEFAULT_ROLE" || f.Name == "ACTIVE" {
			t.Errorf("class constant %q should not be extracted as a field", f.Name)
		}
	}
	if len(fields) != 1 || fields[0].Name != "email" {
		t.Errorf("want only email field, got %v", fields)
	}
}

func TestParseModels_MultipleInheritanceWithMixin(t *testing.T) {
	src := []byte(`
from django.db import models

class AuditMixin:
    pass

class Order(AuditMixin, models.Model):
    total = models.DecimalField(max_digits=10, decimal_places=2)
`)

	fields, err := ParseModels(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := false
	for _, f := range fields {
		if f.Model == "Order" && f.Name == "total" {
			found = true
		}
	}
	if !found {
		t.Errorf("Order.total not found; got %v", fields)
	}
}

func TestParseModels_ThirdPartyFields(t *testing.T) {
	src := []byte(`
from django.db import models
from imagekit.models import ImageSpecField
from django_extensions.db.fields import AutoSlugField

class Article(models.Model):
    thumbnail = ImageSpecField(source="photo")
    slug = AutoSlugField(populate_from="title")
    title = models.CharField(max_length=200)
`)

	fields, err := ParseModels(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	names := map[string]bool{}
	for _, f := range fields {
		names[f.Name] = true
	}
	if !names["title"] {
		t.Error("title (models.CharField) should be extracted")
	}
	if !names["thumbnail"] {
		t.Error("thumbnail (ImageSpecField) should be extracted — ends in Field")
	}
	if !names["slug"] {
		t.Error("slug (AutoSlugField) should be extracted — ends in Field")
	}
}

func TestParseModels_DirectImport(t *testing.T) {
	src := []byte(`
from django.db.models import CharField, EmailField

class User(models.Model):
    name = CharField(max_length=100)
    email = EmailField()
`)

	fields, err := ParseModels(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	names := map[string]bool{}
	for _, f := range fields {
		names[f.Name] = true
	}
	if !names["name"] {
		t.Error("name (CharField bare identifier) should be extracted")
	}
	if !names["email"] {
		t.Error("email (EmailField bare identifier) should be extracted")
	}
}

func TestParseModels_ThirdPartyAttributeField(t *testing.T) {
	// Field accessed as attribute (e.g. imagekit.ImageSpecField) rather than
	// bare identifier. Exercises isDjangoField's attribute branch when
	// obj != "models" but attr ends in "Field".
	src := []byte(`
from django.db import models
import imagekit

class Article(models.Model):
    thumbnail = imagekit.ImageSpecField(source="photo")
    title = models.CharField(max_length=200)
`)

	fields, err := ParseModels(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	names := map[string]bool{}
	for _, f := range fields {
		names[f.Name] = true
	}
	if !names["thumbnail"] {
		t.Error("thumbnail (imagekit.ImageSpecField) should be extracted — attr ends in Field")
	}
	if !names["title"] {
		t.Error("title (models.CharField) should be extracted")
	}
}

func TestParseModels_NonFieldAttribute_NotDetected(t *testing.T) {
	// Exercises isDjangoField's attribute branch when obj != "models" AND
	// attr does NOT end in "Field" — should return false.
	src := []byte(`
from django.db import models

class Invoice(models.Model):
    amount = accounting.MoneyColumn(currency="JPY")
    note = models.TextField()
`)

	fields, err := ParseModels(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	names := map[string]bool{}
	for _, f := range fields {
		names[f.Name] = true
	}
	if names["amount"] {
		t.Error("amount (accounting.MoneyColumn) should NOT be extracted")
	}
	if !names["note"] {
		t.Error("note (models.TextField) should be extracted")
	}
}

func TestParseModels_UnknownCustomField_NotDetected(t *testing.T) {
	src := []byte(`
from django.db import models
from myapp.db import MoneyColumn

class Invoice(models.Model):
    amount = MoneyColumn(currency="JPY")
    note = models.TextField()
`)

	fields, err := ParseModels(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	names := map[string]bool{}
	for _, f := range fields {
		names[f.Name] = true
	}
	if names["amount"] {
		t.Errorf("amount (MoneyColumn, no 'Field' suffix) should NOT be extracted — v0.1 limitation")
	}
	if !names["note"] {
		t.Error("note (models.TextField) should be extracted")
	}
}

func TestParseModels_NonModelSuperclasses_NotIncluded(t *testing.T) {
	// isModelClass must iterate all superclasses and return false when none
	// end in "Model". Exercises the loop-exhausted return false path.
	src := []byte(`
from django.db import models

class SomeHelper(BaseHelper, Mixin):
    name = models.CharField(max_length=100)

class RealModel(models.Model):
    value = models.IntegerField()
`)

	fields, err := ParseModels(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, f := range fields {
		if f.Model == "SomeHelper" {
			t.Errorf("SomeHelper (no Model base) should not be included; got field %v", f)
		}
	}
	found := false
	for _, f := range fields {
		if f.Model == "RealModel" && f.Name == "value" {
			found = true
		}
	}
	if !found {
		t.Errorf("RealModel.value not found; got %v", fields)
	}
}

func TestParseModels_TupleAssignmentSkipped(t *testing.T) {
	// Tuple unpacking on the left side is not a field declaration.
	// Exercises the left.Type() != "identifier" continue branch in extractFields.
	src := []byte(`
from django.db import models

class User(models.Model):
    email = models.EmailField()
    (a, b) = (1, 2)
`)

	fields, err := ParseModels(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, f := range fields {
		if f.Name == "a" || f.Name == "b" {
			t.Errorf("tuple assignment target %q should not be extracted as a field", f.Name)
		}
	}
	if len(fields) != 1 || fields[0].Name != "email" {
		t.Errorf("want only email field, got %v", fields)
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

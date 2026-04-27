package parser

import (
	"testing"
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

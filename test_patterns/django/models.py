from django.db import models


class Author(models.Model):
    name = models.CharField(max_length=255)


class Article(models.Model):
    title = models.CharField(max_length=255)
    slug = models.SlugField(unique=True)
    email = models.EmailField()
    status = models.CharField(max_length=20)
    author = models.ForeignKey(Author, on_delete=models.CASCADE)

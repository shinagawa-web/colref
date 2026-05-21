from django.db import models


class Article(models.Model):
    title = models.CharField(max_length=255)
    slug = models.SlugField(unique=True)
    email = models.EmailField()
    status = models.CharField(max_length=20)

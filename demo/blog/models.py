from django.db import models


class Article(models.Model):
    title = models.CharField(max_length=200)
    slug = models.SlugField(unique=True)
    seo_title = models.CharField(max_length=200, blank=True)
    body = models.TextField()
    published_at = models.DateTimeField(null=True, blank=True)

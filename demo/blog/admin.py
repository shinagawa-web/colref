from django.contrib import admin

from .models import Article


@admin.register(Article)
class ArticleAdmin(admin.ModelAdmin):
    list_display = ("title", "slug", "published_at")
    search_fields = ("title", "seo_title")

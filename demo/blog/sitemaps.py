from django.contrib.sitemaps import Sitemap

from .models import Article


class ArticleSitemap(Sitemap):
    def items(self):
        return Article.objects.all()

    def title(self, obj):
        return obj.seo_title or obj.title

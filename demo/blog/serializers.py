from rest_framework import serializers

from .models import Article


class ArticleSerializer(serializers.ModelSerializer):
    class Meta:
        model = Article
        fields = ["id", "title", "slug", "seo_title", "body"]

    def get_display_title(self, obj):
        return obj.seo_title or obj.title

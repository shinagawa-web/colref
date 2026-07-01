from django.shortcuts import render, get_object_or_404

from .models import Article


def article_detail(request, slug):
    article = get_object_or_404(Article, slug=slug)
    page_title = article.seo_title or article.title
    return render(request, "article.html", {"article": article, "page_title": page_title})

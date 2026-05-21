"""
Synthetic pattern battery for Article.title field reference detection.
Each pattern is on its own line for precise line-level verification.
"""
from django.contrib import admin
from django.db.models import F, Q, Max, Value, OuterRef
from django.db.models.functions import Coalesce, Concat
import operator


# ── Django admin ──────────────────────────────────────────────────────────────
# Kept at module level: this is where ModelAdmin subclasses live in real code.
class ArticleAdmin(admin.ModelAdmin):
    list_display = ['title']                            # list_display
    list_filter = ['title']                             # list_filter
    search_fields = ['title']                           # search_fields
    readonly_fields = ['title']                         # readonly_fields
    fieldsets = (None, {'fields': ['title']})           # fieldsets
    ordering = ['title']                                # ordering


# All remaining patterns are wrapped in `if False` so this file is safe to
# import by tooling. The AST is still fully parseable by colref.
if False:
    article = None
    qs = None
    field_name = None

    # ── Attribute access — read ───────────────────────────────────────────────
    x = article.title                                       # direct access
    x = qs.first().title                                    # chained call
    x = (Article.objects
         .get(pk=1)
         .title)                                            # multi-line chain
    x = f"{article.title}"                                  # inside f-string
    x = article.title if article else ""                    # conditional
    x = [a.title for a in qs]                              # list comprehension

    # ── Attribute access — write ──────────────────────────────────────────────
    article.title = "new"                                   # assignment
    article.title += " suffix"                             # augmented assignment

    # ── getattr / attrgetter ──────────────────────────────────────────────────
    x = getattr(article, 'title')                          # getattr literal
    x = getattr(article, 'title', '')                      # getattr with default
    x = operator.attrgetter('title')(article)              # attrgetter
    x = getattr(article, field_name)                       # variable (not detectable)

    # ── ORM — lookup methods ──────────────────────────────────────────────────
    qs = Article.objects.filter(title='x')                 # filter keyword
    qs = Article.objects.filter(title__icontains='x')      # filter lookup
    qs = Article.objects.exclude(title='x')                # exclude
    obj = Article.objects.get(title='x')                   # get
    obj, _ = Article.objects.get_or_create(title='x')      # get_or_create
    obj, _ = Article.objects.update_or_create(defaults={'title': 'x'}, slug='s')  # update_or_create
    qs = Article.objects.filter(Q(title='x'))              # Q()
    qs = Article.objects.filter(Q(title__in=['x']))        # Q() with lookup

    # ── ORM — queryset methods ────────────────────────────────────────────────
    qs = Article.objects.values('title')                   # values
    qs = Article.objects.values_list('title', flat=True)   # values_list
    qs = Article.objects.only('title')                     # only
    qs = Article.objects.defer('title')                    # defer
    qs = Article.objects.order_by('title')                 # order_by asc
    qs = Article.objects.order_by('-title')                # order_by desc
    qs = Article.objects.select_related('author')          # select_related (FK field)
    obj = Article.objects.latest('title')                  # latest
    obj = Article.objects.earliest('title')                # earliest
    qs = Article.objects.distinct('title')                 # distinct (PostgreSQL)
    obj = Article.objects.create(title='x')                # create
    Article.objects.update(title='x')                      # update (bulk)

    # ── ORM — expressions ─────────────────────────────────────────────────────
    expr = F('title')                                       # F()
    qs = Article.objects.annotate(t=F('title'))            # annotate with F
    x = Article.objects.aggregate(Max('title'))            # aggregate
    expr = Coalesce('title', Value(''))                    # Coalesce
    expr = Concat('title', Value(' - '))                   # Concat
    expr = OuterRef('title')                               # OuterRef

    # ── save with update_fields ───────────────────────────────────────────────
    article.save(update_fields=['title'])                  # update_fields

    # ── Meta API ──────────────────────────────────────────────────────────────
    f = Article._meta.get_field('title')                   # _meta.get_field
    x = vars(article)['title']                             # vars() subscript
    x = article.__dict__['title']                         # __dict__ subscript

    # ── Django REST Framework ─────────────────────────────────────────────────
    fields = ['title', 'slug']                             # Meta.fields list
    extra_kwargs = {'title': {'required': False}}          # extra_kwargs key

    # ── Django forms ──────────────────────────────────────────────────────────
    form_fields = ['title']                                # ModelForm.Meta.fields

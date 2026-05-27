---
title: Detection patterns
weight: 40
---

# Detection patterns

colref uses static AST analysis. It can only detect patterns where the field name appears as a literal in the source. References where the field name is constructed at runtime (e.g. `getattr(obj, field_name)`) are out of scope by design — static analysis cannot determine what string `field_name` holds.

This page documents exactly which patterns are and are not detected for each ORM. The ground truth is the golden test files in `test_patterns/`.

## Output labels

All three mean the reference was detected. The label indicates how it was found and how confident the match is.

| Result | How found | Confidence |
|--------|-----------|------------|
| ✅ | AST attribute node (`article.title`) | Highest — unambiguous |
| ✅ `[string]` | Literal string or symbol passed to a known ORM method (`.where(title: value)`, `.pluck(:title)`) | High — method is known to accept field names |
| ✅ `[sql ref]` | Word-boundary substring match inside a raw SQL string (`.where("title = ?", value)`) | Lower — verify manually, false positives possible |

## Django {#django}

### Detected

<details>
<summary>Attribute access</summary>

| Pattern | Example | Result |
|---------|---------|--------|
| Read | `article.title` | ✅ |
| Chained | `qs.first().title` | ✅ |
| Multi-line chain | `Article.objects.get(pk=1).title` | ✅ |
| Inside f-string | `f"{article.title}"` | ✅ |
| Conditional | `article.title if article else ""` | ✅ |
| List comprehension | `[a.title for a in qs]` | ✅ |
| Write | `article.title = value` | ✅ |
| Augmented write | `article.title += " suffix"` | ✅ |

colref makes no read/write distinction — both are matched as attribute nodes.

</details>

<details>
<summary>ORM — keyword argument methods</summary>

The field name appears as a keyword argument. Lookup suffixes (`__icontains`, `__in`, etc.) are stripped before matching.

| Method | Example | Result |
|--------|---------|--------|
| `filter` | `.filter(title="x")`, `.filter(title__icontains="x")` | ✅ `[string]` |
| `exclude` | `.exclude(title="x")` | ✅ `[string]` |
| `get` | `.get(title="x")` | ✅ `[string]` |
| `get_or_create` | `.get_or_create(title="x")` | ✅ `[string]` |
| `create` | `.create(title="x")` | ✅ `[string]` |
| `update` (bulk) | `.update(title="x")` | ✅ `[string]` |
| `Q` | `Q(title="x")`, `Q(title__in=["x"])` | ✅ `[string]` |
| `annotate` | `.annotate(alias=expr)` (keyword name) | ✅ `[string]` |

</details>

<details>
<summary>ORM — positional string argument methods</summary>

The field name appears as a positional string argument. For `order_by`, a leading `-` is stripped before matching.

| Method | Example | Result |
|--------|---------|--------|
| `values` | `.values("title")` | ✅ `[string]` |
| `values_list` | `.values_list("title", flat=True)` | ✅ `[string]` |
| `only` | `.only("title")` | ✅ `[string]` |
| `defer` | `.defer("title")` | ✅ `[string]` |
| `order_by` (asc) | `.order_by("title")` | ✅ `[string]` |
| `order_by` (desc) | `.order_by("-title")` | ✅ `[string]` |
| `select_related` | `.select_related("author")` | ✅ `[string]` |
| `prefetch_related` | `.prefetch_related("author")` | ✅ `[string]` |
| `latest` | `.latest("title")` | ✅ `[string]` |
| `earliest` | `.earliest("title")` | ✅ `[string]` |
| `distinct` (PostgreSQL) | `.distinct("title")` | ✅ `[string]` |

</details>

<details>
<summary>ORM — expression and aggregate functions</summary>

The field name appears as the first positional string argument.

| Function | Example | Result |
|----------|---------|--------|
| `F` | `F("title")`, `.annotate(t=F("title"))` | ✅ `[string]` |
| Aggregates | `Max("title")`, `Min("title")`, `Avg("title")`, `Sum("title")`, `Count("title")`, `StdDev("title")`, `Variance("title")` | ✅ `[string]` |
| Database functions | `Coalesce("title", Value(""))`, `Concat("title", Value(" "))`, `Greatest("title", ...)`, `Least("title", ...)`, `NullIf("title", ...)` | ✅ `[string]` |
| Subquery | `OuterRef("title")`, `Subquery(...)` | ✅ `[string]` |

</details>

<details>
<summary>Raw SQL</summary>

| Method | Example | Result |
|--------|---------|--------|
| `.raw()` | `Article.objects.raw("SELECT title FROM ...")` | ✅ `[sql ref]` |
| `cursor.execute()` | `cursor.execute("SELECT title, slug FROM ...")` | ✅ `[sql ref]` |

</details>

### Not detected

<details>
<summary>getattr / attrgetter</summary>

| Pattern | Example | Result |
|---------|---------|--------|
| `getattr` with literal | `getattr(article, "title")` | ❌ |
| `getattr` with default | `getattr(article, "title", "")` | ❌ |
| `attrgetter` | `attrgetter("title")(article)` | ❌ |
| `getattr` with variable | `getattr(article, field_name)` | ❌ out of scope by design |

</details>

<details>
<summary>ORM — uncovered methods</summary>

| Pattern | Example | Result |
|---------|---------|--------|
| `update_or_create` | `.update_or_create(defaults={"title": "x"})` | ❌ |
| `save` with `update_fields` | `article.save(update_fields=["title"])` | ❌ |

</details>

<details>
<summary>Meta API</summary>

| Pattern | Example | Result |
|---------|---------|--------|
| `_meta.get_field` | `Article._meta.get_field("title")` | ❌ |
| `vars()` subscript | `vars(article)["title"]` | ❌ |
| `__dict__` subscript | `article.__dict__["title"]` | ❌ |

</details>

<details>
<summary>Django admin</summary>

| Pattern | Example | Result |
|---------|---------|--------|
| `list_display` | `list_display = ["title"]` | ❌ |
| `list_filter` | `list_filter = ["title"]` | ❌ |
| `search_fields` | `search_fields = ["title"]` | ❌ |
| `readonly_fields` | `readonly_fields = ["title"]` | ❌ |
| `fieldsets` | `fieldsets = (None, {"fields": ["title"]})` | ❌ |
| `ordering` | `ordering = ["title"]` | ❌ |

</details>

<details>
<summary>Django REST Framework</summary>

| Pattern | Example | Result |
|---------|---------|--------|
| `Meta.fields` | `fields = ["title", "slug"]` | ❌ |
| `extra_kwargs` | `extra_kwargs = {"title": {...}}` | ❌ |

</details>

<details>
<summary>Django forms</summary>

| Pattern | Example | Result |
|---------|---------|--------|
| `ModelForm.Meta.fields` | `fields = ["title"]` | ❌ |

</details>

---

## Rails {#rails}

### Detected

<details>
<summary>Attribute access</summary>

| Pattern | Example | Result |
|---------|---------|--------|
| Read | `article.title` | ✅ |
| Chained | `Article.find(1).title` | ✅ |
| Multi-line chain | `Article.where(...).first.title` | ✅ |
| String interpolation | `"#{article.title}"` | ✅ |
| Safe navigation | `article&.title` | ✅ |
| Write | `article.title = value` | ✅ |

</details>

<details>
<summary>ActiveRecord — creation</summary>

| Method | Example | Result |
|--------|---------|--------|
| `new` | `Article.new(title: value)` | ✅ `[string]` |
| `create` | `Article.create(title: value)` | ✅ `[string]` |
| `find_or_create_by` | `Article.find_or_create_by(title: value)` | ✅ `[string]` |
| `find_or_initialize_by` | `Article.find_or_initialize_by(title: value)` | ✅ `[string]` |

</details>

<details>
<summary>ActiveRecord — instance update</summary>

| Method | Example | Result |
|--------|---------|--------|
| `update` | `article.update(title: value)` | ✅ `[string]` |
| `assign_attributes` | `article.assign_attributes(title: value)` | ✅ `[string]` |
| `update_column` (symbol) | `article.update_column(:title, value)` | ✅ `[string]` |
| `update_columns` (hash) | `article.update_columns(title: value)` | ✅ `[string]` |

</details>

<details>
<summary>ActiveRecord — query methods</summary>

| Method | Example | Result |
|--------|---------|--------|
| `where` (hash) | `.where(title: value)` | ✅ `[string]` |
| `where` (string) | `.where("title = ?", value)` | ✅ `[sql ref]` |
| `where.not` | `.where.not(title: value)` | ✅ `[string]` |
| `find_by` | `.find_by(title: value)` | ✅ `[string]` |
| `exists?` | `.exists?(title: value)` | ✅ `[string]` |
| `order` (symbol) | `.order(:title)` | ✅ `[string]` |
| `order` (hash) | `.order(title: :desc)` | ✅ `[string]` |
| `order` (string) | `.order("title ASC")` | ✅ `[sql ref]` |
| `pluck` (symbol) | `.pluck(:title)` | ✅ `[string]` |
| `pluck` (string) | `.pluck("title")` | ✅ `[string]` |
| `select` (symbol) | `.select(:title)` | ✅ `[string]` |
| `select` (string) | `.select("title, slug")` | ✅ `[sql ref]` |
| `group` | `.group(:title)` | ✅ `[string]` |
| `pick` | `.pick(:title)` | ✅ `[string]` |
| `reorder` | `.reorder(:title)` | ✅ `[string]` |
| `update_all` | `.update_all(title: value)` | ✅ `[string]` |

</details>

<details>
<summary>ActiveRecord — aggregation</summary>

| Method | Example | Result |
|--------|---------|--------|
| `minimum` | `.minimum(:title)` | ✅ `[string]` |
| `maximum` | `.maximum(:title)` | ✅ `[string]` |
| `sum` | `.sum(:title)` | ✅ `[string]` |

</details>

<details>
<summary>Arel</summary>

| Pattern | Example | Result |
|---------|---------|--------|
| Table subscript | `Article.arel_table[:title]` | ✅ `[string]` |
| Arel condition | `Article.arel_table[:title].eq(value)` | ✅ `[string]` |
| Implicit self | `arel_table[:title]` | ✅ `[string]` |

</details>

<details>
<summary>Raw SQL</summary>

| Method | Example | Result |
|--------|---------|--------|
| `find_by_sql` (string) | `Article.find_by_sql("SELECT title FROM articles ...")` | ✅ `[sql ref]` |
| `find_by_sql` (heredoc) | `Article.find_by_sql(<<~SQL)` with `title` in body | ✅ `[sql ref]` |
| `execute` | `connection.execute("UPDATE articles SET title = ...")` | ✅ `[sql ref]` |
| `select_all` | `connection.select_all("SELECT title, slug FROM articles")` | ✅ `[sql ref]` |

</details>

<details>
<summary>Model declarations (partial)</summary>

The `scope` declaration itself is not matched, but calls inside the scope body are scanned normally.

| Pattern | Example | Result |
|---------|---------|--------|
| `scope` (body) | `scope :titled, ->(t) { where(title: t) }` — detected via `where` | ✅ `[string]` |

</details>

### Not detected

<details>
<summary>Hash / symbol access</summary>

| Pattern | Example | Result |
|---------|---------|--------|
| Symbol subscript | `article[:title]` | ❌ |
| `read_attribute` | `article.read_attribute(:title)` | ❌ |
| `write_attribute` | `article.write_attribute(:title, value)` | ❌ |
| `send` | `article.send(:title)` | ❌ |
| `public_send` | `article.public_send(:title)` | ❌ |

</details>

<details>
<summary>Model declarations</summary>

| Pattern | Example | Result |
|---------|---------|--------|
| `validates` | `validates :title, presence: true` | ❌ |
| `scope` (declaration) | `scope :titled, ->(t) { ... }` | ❌ |

</details>

<details>
<summary>Serialization / presentation</summary>

| Pattern | Example | Result |
|---------|---------|--------|
| Strong params `permit` | `params.require(:article).permit(:title, :slug)` | ❌ |
| ActiveModel Serializer `attributes` | `attributes :title, :slug` | ❌ |

</details>

<details>
<summary>Dynamic / metaprogramming</summary>

| Pattern | Example | Result |
|---------|---------|--------|
| `respond_to?` | `article.respond_to?(:title)` | ❌ |
| `instance_variable_get` | `article.instance_variable_get(:@title)` | ❌ |
| `attribute_changed?` | `article.title_changed?` | ❌ |
| Dynamic finder | `Article.find_by_title(value)` | ❌ |

</details>

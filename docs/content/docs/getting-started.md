---
title: Getting started
weight: 10
---

# Getting started

## Installation

Binaries for Linux and macOS are available on the [releases page](https://github.com/shinagawa-web/colref/releases).

Download the archive for your platform, extract it, and place the `colref` binary somewhere on your `PATH`.

**macOS / Linux (example)**

```sh
curl -L https://github.com/shinagawa-web/colref/releases/latest/download/colref_linux_amd64.tar.gz | tar xz
mv colref /usr/local/bin/
```

Verify the install:

```sh
colref --version
```

## First run

### Django project

From your project root, run:

```sh
colref check --orm django --model Article --field title
```

colref finds all `models.py` files automatically, validates that `Article` has a `title` field, then walks your codebase reporting every live reference.

**No references found:**

```
Scanning 87 files...

No references found for Article.title

  Verify manually before deleting.
```

**References found:**

```
Scanning 87 files...

References found for Article.title

  blog/views.py:14     article.title
  blog/views.py:55     article.title
  blog/serializers.py:9  ✅ [string] .values("title")
  blog/admin.py:22     ❌ list_display = ["title"]   (not detected)
```

### Rails project

From your project root, run:

```sh
colref check --orm rails --model Article --field title
```

colref reads `db/schema.rb` (or falls back to replaying `db/migrate/`) to validate the field exists, then scans `.rb` and `.erb` files for references.

## Next steps

- [Usage]({{< relref "usage/_index" >}}) — all flags, ORM-specific behavior
- [Detection patterns]({{< relref "detection-patterns" >}}) — full breakdown of what is and isn't matched
- [Limitations]({{< relref "limitations" >}}) — what static analysis can't cover

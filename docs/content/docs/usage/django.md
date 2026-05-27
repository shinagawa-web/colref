---
title: Django
weight: 21
---

# Django

## Example

The following examples use [wagtail](https://github.com/wagtail/wagtail), a popular Django CMS.

**No references found:**

```
$ colref check --orm django --model Page --field search_description
Scanning 932 files...

No references found for Page.search_description

  Verify manually before deleting.
```

**References found:**

```
$ colref check --orm django --model Page --field seo_title
Scanning 932 files...

References found for Page.seo_title

  wagtail/admin/tests/pages/test_create_page.py:1867   page.seo_title
  wagtail/admin/tests/pages/test_create_page.py:1892   page.seo_title
```

## models.py detection

colref locates `models.py` automatically by walking the target directory. All `models.py` files found are parsed and merged.

If the same model name appears in more than one `models.py`, colref exits with an error and lists the conflicting files:

```
model "User" found in multiple files:
  accounts/models.py
  legacy/models.py
Use --model to disambiguate.
```

## Skipped directories

The following directories are never scanned:

- `.git`, and any directory whose name starts with `.`
- `__pycache__`
- `venv`, `.venv`
- `migrations`
- `node_modules`

## Detection coverage

See [Detection patterns — Django]({{< relref "detection-patterns#django" >}}) for the full per-pattern breakdown.

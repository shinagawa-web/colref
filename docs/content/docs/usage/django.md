---
title: Django
weight: 21
---

# Django

See [Getting started]({{< relref "getting-started" >}}) for usage examples.

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

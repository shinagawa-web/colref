---
title: Django
weight: 21
---

# Django

## Example

```sh
colref check --orm django --model User --field email
```

Output when no references exist:

```
Scanning 142 files...

No references found for User.email

  Verify manually before deleting.
```

Output when references exist:

```
Scanning 142 files...

References found for User.email

  accounts/serializers.py:34   user.email
  accounts/views.py:88         obj.email
  notifications/tasks.py:12    instance.email
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

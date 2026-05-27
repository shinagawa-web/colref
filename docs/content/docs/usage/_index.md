---
title: Usage
weight: 20
bookFlatSection: true
---

# Usage

## Command

```
colref check --orm <orm> --model <Model> --field <field> [path]
```

`path` is the project root to scan (default: current directory).

## Flags

| Flag | Description |
|---|---|
| `--orm` | ORM type: `django`, `rails` (required) |
| `--model` | Model name to look up (required) |
| `--field` | Field name to search for (required) |

## ORM-specific behavior

- [Django]({{< relref "django" >}}) — models.py detection, skipped directories
- [Rails]({{< relref "rails" >}}) — schema.rb, migration fallback, scanned file types

---
title: How it works
weight: 30
---

# How it works

## Overview

1. Reads your ORM schema source to extract the field list
2. Walks the codebase and parses each file into an AST
3. Reports every location where the field name appears as an attribute access (e.g. `user.email`)

AST parsing avoids false positives from comments, migration files, and unrelated string matches that plain grep would surface.

## Schema extraction

**Django** — colref walks the target directory to find `models.py` files. Each file is parsed and the field list for the requested model is extracted. If the same model name appears in multiple files, colref exits with an error.

**Rails** — colref reads `db/schema.rb` and maps table names to model names by convention (`users` → `User`). If `db/schema.rb` is absent, colref falls back to replaying `db/migrate/` in timestamp order, applying `create_table`, `add_column`, `remove_column`, `rename_column`, and `drop_table` operations to reconstruct the live schema.

## AST scanning

After schema extraction, colref walks the project tree. For each source file, it:

1. Parses the file into an AST
2. Visits every node looking for:
   - **Attribute access** — `object.field` nodes (highest confidence)
   - **ORM keyword arguments** — `.filter(field="x")`, `.where(field: value)`, etc. (`[string]` label)
   - **Raw SQL strings** — word-boundary substring match inside SQL literals (`[sql ref]` label)

Directories listed in the skip list (`.git`, `__pycache__`, `migrations`, `node_modules`, etc.) are never entered.

## Output labels

| Label | How found | Confidence |
|-------|-----------|------------|
| (none) | AST attribute node (`article.title`) | Highest — unambiguous |
| `[string]` | Literal string or symbol passed to a known ORM method | High — method is known to accept field names |
| `[sql ref]` | Word-boundary match inside a raw SQL string | Lower — verify manually, false positives possible |

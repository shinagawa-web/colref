# Architecture

This document describes the internal design of colref for contributors and maintainers.
For user-facing documentation, see [README.md](README.md).

## 1. Pipeline overview

```
Schema source
  └─ Django: models.py files  ──► BuildModelSet ──► ParseModelsWithSet ──► []Field
  └─ Rails:  db/schema.rb         ParseSchemaRb ──────────────────────────► []Field
             db/migrate/ (fallback) ParseMigrations
                                                         │
                                                         ▼
                                              field membership check
                                                         │
                                                         ▼
                                              Reference scanner
                                  Django: attribute nodes + string-based ORM refs
                                  Rails:  method call nodes + string-based AR refs
                                                         │
                                                         ▼
                                              []Reference  (sorted, per-line deduped)
```

Entry point: `cmd/colref/check.go:runCheck` dispatches to `runCheckDjango` or `runCheckRails`.

## 2. Per-ORM schema extraction

### Django

Schema extraction runs in two phases (`cmd/colref/check.go:runCheckDjango`):

1. **Build model set** — `internal/schema/django.go:BuildModelSet` parses all model files with the Python grammar and computes which class names are Django models (see §3).
2. **Extract fields** — `internal/schema/django.go:ParseModelsWithSet` re-parses each file and pulls field assignments from model classes (see §4).

### Rails

`cmd/colref/check.go:runCheckRails` tries `db/schema.rb` first:

- **`db/schema.rb` present** — `internal/schema/rails_schema.go:ParseSchemaRb` walks `create_table` blocks. The model name is derived from the table name via a singularize + CamelCase heuristic (`tableToModel`; irregular plurals are not handled).
- **`db/schema.rb` absent** — `internal/schema/rails_migrations.go:ParseMigrations` reads all `.rb` files under `db/migrate/` in filename order (the timestamp prefix ensures chronological order) and replays `create_table`, `add_column`, `remove_column`, `rename_column`, and `drop_table` operations to reconstruct the current schema.

`db/structure.sql` as an intermediate source between the two is tracked in [#97](https://github.com/shinagawa-web/colref/issues/97) (not yet implemented).

## 3. Model detection (Django)

`internal/schema/django.go:extractClassEntries` classifies each top-level `class_definition` by its superclasses:

- `models.Model` or any `x.Model` attribute → direct Django seed
- bare `Model` identifier → direct Django seed (covers `from django.db.models import Model`)
- any other name → recorded for transitive resolution

`internal/schema/django.go:computeModelSet` then runs an iterative fixed-point loop: a class becomes a model if any of its superclass names is already in the model set. The loop repeats until no new classes are added. This handles arbitrary inheritance depth across multiple files.

## 4. Field detection heuristic (Django)

`internal/schema/django.go:isDjangoField` accepts an assignment's right-hand side if:

- it is a call or attribute rooted at `models.*` (e.g., `models.CharField(...)`), **or**
- the attribute or identifier name ends in `Field` (covers third-party fields and direct imports like `CharField`)

## 5. File discovery vs. reference scanning

These two walks cover different sets of files, by design.

**Model file discovery** (`cmd/colref/check.go:findModelsFiles`) — Django only:

- Matches `models.py`, `abstract_models.py`, and any `*.py` directly under a `models/` directory.
- Applies `refs.SkipDirs` (see §7).

**Reference scanning** (`internal/refs/scan.go:scanFiles`):

- Walks **all** `.py` / `.rb` / `.erb` files in the project tree.
- Each file extension can carry an optional source transform (used for ERB, see §6). The transform must preserve `len(src)` so that tree-sitter byte offsets remain valid.
- Per-file results are deduped by line before merging (`dedupeByLine`).

Django scanning (`internal/refs/django.go:ScanDjango`) runs two passes — attribute nodes and string-based ORM references — then merges and sorts.
Rails scanning (`internal/refs/rails.go:ScanRuby`) runs both in a single walk per file.

## 6. Language-specific notes

### ERB transformation

`internal/refs/rails.go:erbToRuby` converts ERB to valid Ruby before parsing:

- HTML outside `<% %>` tags → spaces (newlines preserved to keep line numbers intact).
- `<%= ... %>` and `<% ... %>` content → kept verbatim.
- `%>` is replaced with `; ` rather than `  ` so that adjacent `<%= a %> <%= b %>` tags on the same line produce two separate Ruby statements; without the semicolon, tree-sitter parses the second expression as a continuation of the first and drops its call nodes (see [#64](https://github.com/shinagawa-web/colref/issues/64)).

### Ruby receiver requirement

`internal/refs/rails.go:walkNodeRuby` matches a `call` node only when it has a receiver. Bare calls like `raw(x)` or `send(x)` that happen to share the field name are skipped.

## 7. Skip-dir policy

Directories are skipped before their contents are walked; no files inside are ever opened.

| ORM    | Skipped directories                                        |
|--------|------------------------------------------------------------|
| Django | `__pycache__`, `venv`, `migrations`, `node_modules`        |
| Rails  | `spec`, `test`, `vendor`, `migrate`, `node_modules`        |

Django skips `migrations` because generated migration files reference field names without implying live usage. Rails skips `spec` and `test` for the same reason and `migrate` because `db/migrate/` is read separately during schema extraction, not during reference scanning.

Source: `internal/refs/django.go:SkipDirs`, `internal/refs/rails.go:RubySkipDirs`.

## 8. Known limitations and extension points

| Limitation | Location |
|---|---|
| Rails `self.table_name` overrides not handled; model name derived from table name only | `internal/schema/rails_schema.go:tableToModel` |
| Irregular plurals (e.g., `people` → `Person`) not singularized | `internal/schema/rails_schema.go:singularize` |
| `db/structure.sql` not yet a schema source for Rails | [#97](https://github.com/shinagawa-web/colref/issues/97) |
| Django multi-file model detection requires all model files to be found by `findModelsFiles` | `cmd/colref/check.go:findModelsFiles` |

To add a new ORM target: implement `orm.SchemaParser` and `orm.ReferenceScanner` (`internal/orm/orm.go`), wire a new `--orm` value in `cmd/colref/check.go:runCheck`, and add the language grammar via `go-tree-sitter`.

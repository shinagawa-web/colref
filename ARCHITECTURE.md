# Architecture

This document describes the internal design of colref for contributors and maintainers.
For user-facing documentation, see [README.md](README.md).

## 1. Pipeline overview

```
Schema source → []Field → field membership check → Reference scanner → []Reference
```

Entry point: `cmd/colref/check.go:runCheck` dispatches to `runCheckDjango` or `runCheckRails`.

## 2. Django

### Model file discovery

`cmd/colref/check.go:findModelsFiles` walks the project tree and collects:

- `models.py`
- `abstract_models.py`
- any `*.py` directly under a `models/` directory

Skipped directories: `__pycache__`, `venv`, `migrations`, `node_modules` (`internal/refs/django.go:SkipDirs`). `migrations` is excluded because generated migration files reference field names without implying live usage.

### Model detection

Schema extraction runs in two phases (`cmd/colref/check.go:runCheckDjango`):

1. **Build model set** — `internal/schema/django.go:BuildModelSet` parses all model files and computes which class names are Django models.
2. **Extract fields** — `internal/schema/django.go:ParseModelsWithSet` re-parses each file and pulls field assignments from model classes only.

`internal/schema/django.go:extractClassEntries` classifies each top-level class by its superclasses:

- `models.Model` or any `x.Model` attribute → direct Django seed
- bare `Model` identifier → direct Django seed (covers `from django.db.models import Model`)
- any other name → recorded for transitive resolution

`internal/schema/django.go:computeModelSet` then runs an iterative fixed-point loop: a class becomes a model if any of its superclass names is already in the model set. The loop repeats until no new classes are added, handling arbitrary inheritance depth across multiple files.

### Field detection heuristic

`internal/schema/django.go:isDjangoField` accepts an assignment's right-hand side if:

- it is a call or attribute rooted at `models.*` (e.g., `models.CharField(...)`), **or**
- the attribute or identifier name ends in `Field` (covers third-party fields and direct imports like `CharField`)

### Reference scanning

`internal/refs/django.go:ScanDjango` runs two passes over all `.py` files and merges the results:

- **Attribute nodes** (`walkNode`) — any `attribute` node whose attribute name equals the field name.
- **String-based ORM refs** (`walkNodeStringRefs`) — positional string args to `values`/`defer`/`only`/etc., keyword args to `filter`/`exclude`/`Q`, first arg of `F()` and aggregates, and raw SQL strings passed to `raw()`/`execute()`.

## 3. Rails

### Schema extraction

`cmd/colref/check.go:runCheckRails` tries `db/schema.rb` first:

- **`db/schema.rb` present** — `internal/schema/rails_schema.go:ParseSchemaRb` walks `create_table` blocks. The model name is derived from the table name via a singularize + CamelCase heuristic (`tableToModel`; irregular plurals are not handled, and `self.table_name` overrides are ignored).
- **`db/schema.rb` absent** — `internal/schema/rails_migrations.go:ParseMigrations` reads all `.rb` files under `db/migrate/` in filename order (the timestamp prefix ensures chronological order) and replays `create_table`, `add_column`, `remove_column`, `rename_column`, and `drop_table` to reconstruct the current schema.

`db/structure.sql` as an intermediate source is tracked in [#97](https://github.com/shinagawa-web/colref/issues/97) (not yet implemented).

### Reference scanning

`internal/refs/rails.go:ScanRuby` walks all `.rb` and `.erb` files in a single pass per file and merges two match strategies:

- **Method call nodes** (`walkNodeRuby`) — `call` nodes where the method name equals the field name. A receiver is required to avoid false positives on bare calls like `raw(x)` or `send(x)`.
- **String-based AR refs** (`walkNodeRubyStringRefs`) — symbol args to `select`/`order`/`pluck`/etc., hash key args to `where`/`find_by`/etc., `arel_table[:field]` subscripts, and raw SQL strings.

Skipped directories: `spec`, `test`, `vendor`, `migrate`, `node_modules` (`internal/refs/rails.go:RubySkipDirs`). `spec` and `test` are excluded because test-only references are noise; `migrate` is excluded because `db/migrate/` is read during schema extraction, not reference scanning.

#### ERB transformation

`.erb` files are converted to length-preserving Ruby before parsing (`internal/refs/rails.go:erbToRuby`):

- HTML outside `<% %>` tags → spaces (newlines preserved to keep line numbers intact).
- `<%= ... %>` and `<% ... %>` content → kept verbatim.
- `%>` is replaced with `; ` rather than `  ` so that adjacent `<%= a %> <%= b %>` tags on the same line produce two separate Ruby statements; without the semicolon, tree-sitter parses the second expression as a continuation of the first and drops its call nodes (see [#64](https://github.com/shinagawa-web/colref/issues/64)).

## 4. Common mechanics

### Reference scanning internals

`internal/refs/scan.go:scanFiles` is the shared file walker used by both ORMs. Each file extension can carry an optional source transform (used for ERB above); the transform must preserve `len(src)` so that tree-sitter byte offsets remain valid. Per-file results are deduped by line before merging (`dedupeByLine`).

### Adding a new ORM target

Implement `orm.SchemaParser` and `orm.ReferenceScanner` (`internal/orm/orm.go`), wire a new `--orm` value in `cmd/colref/check.go:runCheck`, and add the language grammar via `go-tree-sitter`.

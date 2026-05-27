# colref

Check whether a database column is still referenced in your codebase before you delete it.

## Why

You want to remove a column from a long-running system. The column looks unused, but you're not sure. A full-text search returns hits inside comments, test fixtures, and migration history — noise that makes it hard to tell whether the column is actually read or written in live code.

colref scans your codebase with an AST parser, skips comments and string literals, and tells you where the column is referenced. If it finds nothing, you have a concrete starting point for the deletion decision. The final call is yours.

## Usage

```
colref check --orm <orm> --model <Model> --field <field> [path]
```

`path` is the project root to scan (default: current directory).

### Django example

```
colref check --orm django --model User --field email
```

Output:

```
Scanning 142 files...

No references found for User.email

  Verify manually before deleting.
```

When references exist:

```
Scanning 142 files...

References found for User.email

  accounts/serializers.py:34   user.email
  accounts/views.py:88         obj.email
  notifications/tasks.py:12    instance.email
```

### Rails example

```
colref check --orm rails --model User --field email
```

colref reads `db/schema.rb` from the project root, infers model names from table names (`users` → `User`), and scans `.rb` and `.erb` files for attribute-access references.

If `db/schema.rb` is not present (some projects treat it as a generated artifact and do not commit it), colref falls back to `db/migrate/`. It replays migration files in timestamp order — `create_table`, `add_column`, `remove_column`, `rename_column`, `drop_table` — to reconstruct the current schema. Model and field validation remain fully intact.

### Flags

| Flag | Description |
|---|---|
| `--orm` | ORM type: `django`, `rails` (required) |
| `--model` | Model name to look up (required) |
| `--field` | Field name to search for (required) |

### Django — models.py detection

colref locates `models.py` automatically by walking the target directory. All `models.py` files found are parsed and merged.

If the same model name appears in more than one `models.py`, colref exits with an error and lists the conflicting files:

```
model "User" found in multiple files:
  accounts/models.py
  legacy/models.py
Use --model to disambiguate.
```

### Skipped directories

The following directories are never scanned:

- `.git`, and any directory whose name starts with `.`
- `__pycache__`
- `venv`, `.venv`
- `migrations`
- `node_modules`

## Installation

Binaries for Linux and macOS will be available on the [releases page](https://github.com/shinagawa-web/colref/releases) once the first version ships.

## How it works

1. Reads your ORM schema source to extract the field list
2. Walks the codebase and parses each file into an AST
3. Reports every location where the field name appears as an attribute access (e.g. `user.email`)

AST parsing avoids false positives from comments, migration files, and unrelated string matches that plain grep would surface.

## Limitations

colref uses static AST analysis and cannot detect every reference pattern. References where the field name is constructed at runtime (e.g. `getattr(obj, field_name)`) are out of scope by design.

**Django:** attribute access, most ORM methods (`filter`, `exclude`, `get`, `Q`, `values`, `only`, `defer`, `order_by`, `F`, aggregates, etc.), and raw SQL strings are detected. Not detected: `getattr` / `attrgetter`, `update_or_create`, `save(update_fields=[...])`, `_meta.get_field`, Django admin class attributes, DRF serializer fields, and form fields.

**Rails:** attribute access, most ActiveRecord query/creation/update methods (`where`, `order`, `pluck`, `create`, `update`, `find_by`, etc.), Arel subscripts, and SQL string fragments are detected. Not detected: `read_attribute`, `send`, symbol subscript (`record[:field]`), `validates` declarations, and strong parameters.

For the full per-pattern breakdown, see [docs/content/docs/detection-patterns.md](docs/content/docs/detection-patterns.md).

If colref reports no references, treat it as "none found by the scanner" — not as a guarantee the column is unused.

## Roadmap

See [issue #74](https://github.com/shinagawa-web/colref/issues/74).

## License

MIT

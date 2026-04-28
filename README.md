# colref

Check whether a database column is still referenced in your codebase before you delete it.

## Why

You want to remove a column from a long-running system. The column looks unused, but you're not sure. A full-text search returns hits inside comments, test fixtures, and migration history — noise that makes it hard to tell whether the column is actually read or written in live code.

colref scans your codebase with an AST parser, skips comments and string literals, and tells you where the column is referenced. If it finds nothing, you have a concrete starting point for the deletion decision. The final call is yours.

## Usage

```
colref check --model User --field email [path]
```

`path` is the directory to scan (default: current directory).

Output:

```
Scanning 142 files...

No references found for User.email

  String-based ORM calls (e.g. .values(), .defer()) are not detected.
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

### Flags

| Flag | Description |
|---|---|
| `--model` | Model name to look up (required) |
| `--field` | Field name to search for (required) |
| `--models-file` | Path to a specific `models.py` (see below) |

### models.py auto-detection

colref locates `models.py` automatically by walking the target directory. All `models.py` files found are parsed and merged.

If the same model name appears in more than one `models.py`, colref exits with an error and lists the conflicting files:

```
model "User" found in multiple files:
  accounts/models.py
  legacy/models.py
Use --models-file to specify which one.
```

Use `--models-file` to point to a specific file and resolve the ambiguity:

```
colref check --model User --field email --models-file accounts/models.py
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

1. Reads your ORM model file to extract the field list
2. Walks the codebase and parses each file into an AST
3. Reports every location where the field name appears as an attribute access (e.g. `user.email`)

AST parsing avoids false positives from comments, migration files, and unrelated string matches that plain grep would surface.

## Limitations

v0.1 detects attribute-access references only (e.g. `user.email`). String-based ORM calls like `.values('email')`, `.defer('email')`, or `Q(email=...)` pass the column name as a string argument and are not yet covered. Support for these patterns will be added in a future version, as it requires per-ORM knowledge of which methods accept column names as strings.

If colref reports no references, treat it as "none found in attribute-access form" — not as a guarantee the column is unused.

## Roadmap

### v0.1 — Django

Fields are declared explicitly in `models.py`, which makes parsing straightforward. The repository alone contains everything colref needs — no database connection required.

### v0.2 — Rails

Schema is consolidated in `schema.rb`. ActiveRecord models follow predictable naming conventions (`User` → `users`), though custom table names need handling.

### v0.3 — Laravel

Schema is spread across many migration files rather than a single source of truth, which makes field extraction more involved compared to Django and Rails.

### Later — Spring / JPA, SQLAlchemy, Entity Framework, TypeORM / Prisma

These will be evaluated after v0.3. Each framework will be scoped separately at that point.

Each version ships only after the previous one is stable. Priorities may shift based on feedback.

## License

MIT

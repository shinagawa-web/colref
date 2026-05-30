---
title: Rails
weight: 22
---

# Rails

See [Getting started]({{< relref "getting-started" >}}) for usage examples.

## Schema source

colref looks for a schema source in the following order:

1. **`db/schema.rb`** — the standard Rails schema dump (`:ruby` format). Present in most projects.
2. **`db/structure.sql`** — used when `config.active_record.schema_format = :sql` (PostgreSQL-specific features such as custom types, partial indexes, or materialized views that `schema.rb` cannot represent). Supports PostgreSQL, MySQL, and SQLite quoting styles.
3. **`db/migrate/`** — fallback when no schema dump is committed. colref replays migration files in timestamp order — `create_table`, `add_column`, `remove_column`, `rename_column`, `drop_table` — to reconstruct the current schema.

Model and field validation are fully intact regardless of which source is used.

## Detection coverage

See [Detection patterns — Rails]({{< relref "detection-patterns#rails" >}}) for the full per-pattern breakdown.

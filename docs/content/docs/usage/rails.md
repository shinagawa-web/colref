---
title: Rails
weight: 22
---

# Rails

## Example

The following examples use [mastodon](https://github.com/mastodon/mastodon).

**No references found:**

```
$ colref check --orm rails --model Account --field sensitized_at
Scanning 1502 files...

No references found for Account.sensitized_at

  Verify manually before deleting.
```

**References found:**

```
$ colref check --orm rails --model Account --field memorial
Scanning 1502 files...

References found for Account.memorial

  app/models/account.rb:176                                 [string] scope :without_memorial, -> { where(memorial: false) }
  app/services/activitypub/process_account_service.rb:149   @account.memorial
  app/services/delete_account_service.rb:231                @account.memorial
```

## Schema source

colref looks for a schema source in the following order:

1. **`db/schema.rb`** — the standard Rails schema dump (`:ruby` format). Present in most projects.
2. **`db/structure.sql`** — used when `config.active_record.schema_format = :sql` (PostgreSQL-specific features such as custom types, partial indexes, or materialized views that `schema.rb` cannot represent). Supports PostgreSQL, MySQL, and SQLite quoting styles.
3. **`db/migrate/`** — fallback when no schema dump is committed. colref replays migration files in timestamp order — `create_table`, `add_column`, `remove_column`, `rename_column`, `drop_table` — to reconstruct the current schema.

Model and field validation are fully intact regardless of which source is used.

## Detection coverage

See [Detection patterns — Rails]({{< relref "detection-patterns#rails" >}}) for the full per-pattern breakdown.

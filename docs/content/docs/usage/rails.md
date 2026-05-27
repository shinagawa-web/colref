---
title: Rails
weight: 22
---

# Rails

## Example

```sh
colref check --orm rails --model User --field email
```

colref reads `db/schema.rb` from the project root, infers model names from table names (`users` → `User`), and scans `.rb` and `.erb` files for attribute-access references.

## Schema source

colref first looks for `db/schema.rb`. If present, it extracts the current column list from there.

If `db/schema.rb` is not present (some projects treat it as a generated artifact and do not commit it), colref falls back to `db/migrate/`. It replays migration files in timestamp order — `create_table`, `add_column`, `remove_column`, `rename_column`, `drop_table` — to reconstruct the current schema. Model and field validation remain fully intact.

## Detection coverage

See [Detection patterns — Rails]({{< relref "detection-patterns#rails" >}}) for the full per-pattern breakdown.

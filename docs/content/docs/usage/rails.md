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

colref first looks for `db/schema.rb`. If present, it extracts the current column list from there.

If `db/schema.rb` is not present (some projects treat it as a generated artifact and do not commit it), colref falls back to `db/migrate/`. It replays migration files in timestamp order — `create_table`, `add_column`, `remove_column`, `rename_column`, `drop_table` — to reconstruct the current schema. Model and field validation remain fully intact.

## Detection coverage

See [Detection patterns — Rails]({{< relref "detection-patterns#rails" >}}) for the full per-pattern breakdown.

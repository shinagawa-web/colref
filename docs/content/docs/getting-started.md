---
title: Getting started
weight: 10
---

# Getting started

## Installation

Binaries for Linux and macOS are available on the [releases page](https://github.com/shinagawa-web/colref/releases).

Download the archive for your platform, extract it, and place the `colref` binary somewhere on your `PATH`.

Verify the install:

```sh
colref --version
```

## First run

### Django project

The following example runs colref against [wagtail](https://github.com/wagtail/wagtail), a popular Django CMS.

**No references found:**

```
$ colref check --orm django --model Page --field search_description
Scanning 932 files...

No references found for Page.search_description

  Verify manually before deleting.
```

**References found:**

```
$ colref check --orm django --model Page --field seo_title
Scanning 932 files...

References found for Page.seo_title

  wagtail/admin/tests/pages/test_create_page.py:1867   page.seo_title
  wagtail/admin/tests/pages/test_create_page.py:1892   page.seo_title
```

### Rails project

The following example runs colref against [mastodon](https://github.com/mastodon/mastodon).

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

## Next steps

- [Usage]({{< relref "usage/_index" >}}) — all flags, ORM-specific behavior
- [Detection patterns]({{< relref "detection-patterns" >}}) — full breakdown of what is and isn't matched
- [Limitations]({{< relref "limitations" >}}) — what static analysis can't cover

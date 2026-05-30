---
title: colref
type: docs
bookToc: false
---

# colref

Check whether a database column is still referenced in your codebase before you delete it.

## Why

You want to remove a column from a long-running system. The column looks unused, but you're not sure. A full-text search returns hits inside comments, test fixtures, and migration history — noise that makes it hard to tell whether the column is actually read or written in live code.

colref scans your codebase with an AST parser, skips comments and string literals, and tells you where the column is referenced. If it finds nothing, you have a concrete starting point for the deletion decision. The final call is yours.

## Quick start

```
colref check --orm django --model User --field email
```

```
Scanning 142 files...

No references found for User.email

  Verify manually before deleting.
```

→ [Installation]({{< relref "docs/installation" >}}) — pip, gem, brew, curl, or manual download  
→ [Getting started]({{< relref "docs/getting-started" >}}) — first run examples  
→ [Usage]({{< relref "docs/usage/_index" >}}) — flags reference and ORM-specific behavior  
→ [Use cases]({{< relref "docs/use-cases" >}}) — schema cleanup, CI integration, audits, and more  
→ [How it works]({{< relref "docs/how-it-works" >}}) — AST parsing and schema extraction  
→ [Detection patterns]({{< relref "docs/detection-patterns" >}}) — what is and isn't detected  

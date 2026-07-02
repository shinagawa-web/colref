---
title: Usage
weight: 20
bookFlatSection: true
---

# Usage

## Command

```
colref check --orm <orm> --model <Model> --field <field> [path]
```

`path` is the project root to scan (default: current directory).

## Flags

| Flag | Description |
|---|---|
| `--orm` | ORM type: `django`, `rails` (required) |
| `--model` | Model name to look up (required) |
| `--field` | Field name to search for (required) |
| `--format` | Output format: `text` (default), `json` |
| `--color` | Colorize `text` output: `auto` (default), `always`, `never` |

## Output formats

### `text` (default)

Human-readable output, including a `Scanning N files...` progress line and an aligned list of `file:line` references.

### `json`

Machine-readable output for scripting and CI. stdout is pure JSON — no `Scanning...` preamble — so it can be piped straight into `jq`:

```
colref check --orm django --model User --field email --format json path/to/project
```

```json
{
  "model": "User",
  "field": "email",
  "orm": "django",
  "files_scanned": 6,
  "reference_count": 1,
  "references": [
    {
      "file": "accounts/views.py",
      "line": 2,
      "text": "user.email"
    }
  ]
}
```

- `references` is always an array — `[]` when nothing matches, never `null`.
- HTML characters are **not** escaped, so source snippets stay readable on the wire — Ruby lambdas (`->`), safe navigation (`&.`), and ERB (`<%= %>`) appear verbatim rather than as `\u003e` / `\u0026`.
- Errors (unknown model/field/orm/format) are still reported on stderr with a non-zero exit code; they are not emitted as JSON.

## Color

The `text` format is colorized for readability: the heading is green when references are found and yellow when none are, file paths are cyan, and `:line` numbers are dimmed. Matched source text is left uncolored so it stands out against the colored metadata.

Color is controlled by `--color`:

- `auto` (default) — colorize only when stdout is a terminal. When the output is piped or redirected, color is disabled automatically, so `text` output stays clean for downstream tools.
- `always` — always colorize, even when piped.
- `never` — never colorize.

The [`NO_COLOR`](https://no-color.org/) environment variable is honored: when it is set to a non-empty value, `auto` disables color. `--color=always` overrides both the TTY check and `NO_COLOR`.

`json` output is never colorized regardless of `--color`.

## ORM-specific behavior

- [Django]({{< relref "django" >}}) — models.py detection, skipped directories
- [Rails]({{< relref "rails" >}}) — schema.rb, migration fallback, scanned file types

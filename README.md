# colref

![Test](https://github.com/shinagawa-web/colref/actions/workflows/test.yml/badge.svg)
[![codecov](https://codecov.io/gh/shinagawa-web/colref/graph/badge.svg)](https://codecov.io/gh/shinagawa-web/colref)
[![Go Report Card](https://goreportcard.com/badge/github.com/shinagawa-web/colref)](https://goreportcard.com/report/github.com/shinagawa-web/colref)
[![Go Reference](https://pkg.go.dev/badge/github.com/shinagawa-web/colref.svg)](https://pkg.go.dev/github.com/shinagawa-web/colref)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Gem Version](https://badge.fury.io/rb/colref.svg)](https://rubygems.org/gems/colref)
[![Gem Downloads](https://img.shields.io/gem/dt/colref.svg)](https://rubygems.org/gems/colref)
[![PyPI version](https://badge.fury.io/py/colref.svg)](https://pypi.org/project/colref/)
[![PyPI Downloads](https://img.shields.io/pypi/dm/colref.svg)](https://pypi.org/project/colref/)

Check whether a database column is still referenced in your codebase before you delete it.

## Why

You want to remove a column from a long-running system. The column looks unused, but you're not sure. A full-text search returns hits inside comments, test fixtures, and migration history — noise that makes it hard to tell whether the column is actually read or written in live code.

colref scans your codebase with an AST parser, skips comments and string literals, and tells you where the column is referenced. If it finds nothing, you have a concrete starting point for the deletion decision. The final call is yours.

## Installation

### pip / pipx (Python users)

If you are working on a Django or Python project, the easiest way to install colref is via pip or pipx. No Go installation required.

```sh
pipx install colref
```

Or with pip:

```sh
pip install colref
```

| OS | x86_64 (Intel/AMD) | arm64 |
|---|---|---|
| macOS | ✓ | ✓ (Apple Silicon) |
| Linux | ✓ | ✓ |
| Windows | ✓ | — |

### gem (Ruby users)

If you are working on a Rails or Ruby project, install via gem. No Go installation required.

```sh
gem install colref
```

### Homebrew (macOS and Linux)

```sh
brew install shinagawa-web/tap/colref
```

If you prefer to tap first:

```sh
brew tap shinagawa-web/tap
brew install colref
```

### One-line installer (Linux and macOS)

```sh
curl -fsSL https://raw.githubusercontent.com/shinagawa-web/colref/main/install.sh | sh
```

### Manual download

Pre-built binaries are available on the [releases page](https://github.com/shinagawa-web/colref/releases).

For full installation options, see [Getting started](https://shinagawa-web.github.io/colref/docs/getting-started/).

## Usage

```
colref check --orm <orm> --model <Model> --field <field> [path]
```

`path` is the project root to scan (default: current directory).

| Flag | Description |
|---|---|
| `--orm` | ORM type: `django`, `rails` (required) |
| `--model` | Model name to look up (required) |
| `--field` | Field name to search for (required) |

Example:

```
$ colref check --orm django --model Page --field seo_title
Scanning 932 files...

References found for Page.seo_title

  wagtail/admin/tests/pages/test_create_page.py:1867   page.seo_title
  wagtail/admin/tests/pages/test_create_page.py:1892   page.seo_title
```

For ORM-specific behavior and more examples, see [Django](https://shinagawa-web.github.io/colref/docs/usage/django/) and [Rails](https://shinagawa-web.github.io/colref/docs/usage/rails/).

## Limitations

colref uses static AST analysis and cannot detect every reference pattern. References where the field name is constructed at runtime (e.g. `getattr(obj, field_name)`) are out of scope by design.

If colref reports no references, treat it as "none found by the scanner" — not as a guarantee the column is unused.

For the full per-pattern breakdown, see [Detection patterns](https://shinagawa-web.github.io/colref/docs/detection-patterns/) and [Limitations](https://shinagawa-web.github.io/colref/docs/limitations/).

## Roadmap

See [issue #74](https://github.com/shinagawa-web/colref/issues/74).

## License

MIT

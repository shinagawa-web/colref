# CLAUDE.md

## External repositories

When cloning OSS projects for verification or accuracy testing, place them under `repos/` at the project root (e.g. `repos/discourse`, `repos/mastodon`). Do **not** use `/tmp` or `/private/tmp`.

The `repos/` directory is listed in `.gitignore` and is never committed.

## GitHub Issues

When creating issues with `gh issue create`, always add relevant labels explicitly with `--label`:

- `orm:django` — issue relates to Django/Python support
- `orm:rails` — issue relates to Rails/Ruby support
- `bug` — bug fix (`fix:` prefix)
- `enhancement` — new feature (`feat:` prefix)
- `test` — test-only change (`test:` prefix)
- `ci` — CI/workflow change (`ci:` prefix)

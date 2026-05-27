# Contributing

## Setup

**Prerequisites:** Go 1.21+

```sh
git clone https://github.com/shinagawa-web/colref.git
cd colref
make install-hooks
```

`make install-hooks` installs a pre-push hook that runs automatically on every `git push`:

1. **Lint** — golangci-lint
2. **Unit tests + coverage** — 100% coverage enforced (e2e excluded)
3. **Benchmarks** — compares against `main` if `benchstat` is installed; otherwise runs without comparison

That covers most changes. Write code, push, the hook tells you if anything is broken.

## Branching and commits

Branch names follow the pattern `<type>/issue-<N>-<short-desc>`:

```
feat/issue-12-rails-support
fix/issue-34-false-positive
docs/issue-85-contributing
ci/issue-91-pin-actions
```

Commit messages use the same type prefix:

```
feat: add Rails schema.rb support
fix: require receiver on Ruby call nodes
docs: add CONTRIBUTING.md
ci: pin actions to SHA
test: add pattern battery for order_by
```

## Running checks manually

```sh
make static-lint      # lint only
make test             # unit tests
make test-e2e         # end-to-end tests
make check-coverage   # unit tests + 100% coverage enforcement
make bench            # benchmarks
```

## Adding a new detection pattern

1. Add the pattern to the relevant scanner in `internal/`
2. Add test cases to `test_patterns/`
3. Run `make update-golden` to regenerate golden files
4. Run `make test-e2e` to verify (the pre-push hook does not run e2e)

## Pull requests

- Open an issue before starting non-trivial work
- Keep PRs focused — one logical change per PR
- All unit tests must pass; coverage must stay at 100%
- e2e tests are required for new detection patterns

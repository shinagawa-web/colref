---
title: Contributing
weight: 60
---

# Contributing

## Setup

**Prerequisites:** Go 1.21+

```sh
git clone https://github.com/shinagawa-web/colref.git
cd colref
make install-hooks
```

`make install-hooks` installs a pre-push hook. Every time you run `git push`, it automatically runs:

1. **Lint** — golangci-lint
2. **Unit tests + coverage** — unit tests with 100% coverage enforcement (e2e excluded)
3. **Benchmarks** — runs benchmarks; compares against `main` if `benchstat` is installed

That's it for most changes. Write code, push, the hook tells you if anything is broken.

## Adding a new detection pattern

1. Add the pattern to the relevant scanner in `internal/`
2. Add test cases to the pattern battery in `test_patterns/`
3. Run `make update-golden` to regenerate golden files
4. Run `make test-e2e` to verify the pattern battery passes (the pre-push hook does not run e2e)
5. Update [Detection patterns]({{< relref "detection-patterns" >}}) with the new pattern
6. Push — the hook runs lint, unit tests, and benchmarks

## Submitting changes

- Open an issue before starting non-trivial work
- Keep PRs focused — one logical change per PR
- All tests must pass; coverage must stay at 100%

## Running checks manually

If you need to run individual checks without pushing:

```sh
make static-lint      # lint only
make test             # unit tests
make test-e2e         # end-to-end tests
make check-coverage   # tests + 100% coverage enforcement
make bench            # benchmarks
make update-golden    # regenerate golden files
```

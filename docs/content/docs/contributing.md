---
title: Contributing
weight: 60
---

# Contributing

## Development setup

**Prerequisites:** Go 1.21+

```sh
git clone https://github.com/shinagawa-web/colref.git
cd colref
make install-hooks
```

## Running tests

```sh
make test           # unit tests (race detector enabled)
make test-e2e       # end-to-end tests
make check-coverage # unit tests + enforce 100% coverage
```

## Linting

```sh
make static-lint    # run golangci-lint
make lint-fix       # run golangci-lint --fix
```

## Building

```sh
make build          # build ./colref binary
make install        # install to $GOPATH/bin
```

## Golden file tests

The pattern battery tests in `e2e/` compare output against golden files in `test_patterns/`. If you add or change a detection pattern, regenerate the golden files:

```sh
make update-golden
```

Golden files must be committed and stay in sync with the implementation. CI enforces this.

## Adding a new detection pattern

1. Add the pattern to the relevant scanner in `internal/`
2. Add test cases to the pattern battery in `test_patterns/`
3. Run `make update-golden` to regenerate golden files
4. Run `make test && make test-e2e` to verify everything passes
5. Update [Detection patterns]({{< relref "detection-patterns" >}}) with the new pattern

## Submitting changes

- Open an issue before starting non-trivial work
- Keep PRs focused — one logical change per PR
- All tests must pass; coverage must stay at 100%

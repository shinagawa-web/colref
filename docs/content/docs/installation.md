---
title: Installation
weight: 5
---

# Installation

| OS | x86_64 (Intel/AMD) | arm64 |
|---|---|---|
| macOS | ✓ | ✓ (Apple Silicon) |
| Linux | ✓ | ✓ |
| Windows | ✓ | — |

## pip / pipx (Python users)

If you are working on a Django or Python project, install via pipx or pip. No Go installation required.

```sh
pipx install colref
```

Or with pip:

```sh
pip install colref
```

## gem (Ruby users)

If you are working on a Rails or Ruby project, install via gem. No Go installation required.

```sh
gem install colref
```

## Homebrew (macOS and Linux)

```sh
brew install shinagawa-web/tap/colref
```

If you prefer to tap first:

```sh
brew tap shinagawa-web/tap
brew install colref
```

## One-line installer (Linux and macOS)

```sh
curl -fsSL https://raw.githubusercontent.com/shinagawa-web/colref/main/install.sh | sh
```

Installs the latest release binary to `/usr/local/bin`. The script detects your OS and architecture automatically, downloads the matching tarball from the [releases page](https://github.com/shinagawa-web/colref/releases), and verifies the SHA-256 checksum before installing.

To install to a different directory, set `INSTALL_DIR`:

```sh
curl -fsSL https://raw.githubusercontent.com/shinagawa-web/colref/main/install.sh | INSTALL_DIR=$HOME/.local/bin sh
```

## Manual download

Pre-built binaries are also available directly on the [releases page](https://github.com/shinagawa-web/colref/releases).

Verify the install:

```sh
colref --version
```

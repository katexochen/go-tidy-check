[![Govulncheck](https://github.com/katexochen/go-tidy-check/actions/workflows/test-govulncheck.yml/badge.svg)](https://github.com/katexochen/go-tidy-check/actions/workflows/test-govulncheck.yml)
[![Integration tests](https://github.com/katexochen/go-tidy-check/actions/workflows/test-integration.yml/badge.svg)](https://github.com/katexochen/go-tidy-check/actions/workflows/test-integration.yml)
[![Golangci-lint](https://github.com/katexochen/go-tidy-check/actions/workflows/test-lint.yml/badge.svg)](https://github.com/katexochen/go-tidy-check/actions/workflows/test-lint.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/katexochen/go-tidy-check)](https://goreportcard.com/report/github.com/katexochen/go-tidy-check)

# 🧹 Check if your modules are tidy

This tool checks whether `go mod tidy` would change your modules `go.mod` or `go.sum` file.
The action can check multiple (sub)modules and will print a diff of the changes that should
be made. It will not make any changes to your modules.

May there be a [`go mod tidy -check`](https://github.com/golang/go/issues/27005) in the future to replace this action.

## 💥 Action

### Usage

```yaml
name: Go mod tidy check
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: katexochen/go-tidy-check@v1
        with:
          # (Optional) The path to the root of each modules, space separated. Default is the current directory.
          modules: . ./submodule ./submodule2
```

## 💻 Command-line interface

### Prerequisites

Having [Go](https://golang.org/doc/install) installed.

### Install

You can install the CLI with the `go` command:

```bash
go install github.com/katexochen/go-tidy-check@latest
```

Or download a binary from the [release page](https://github.com/katexochen/go-tidy-check/releases/latest).

### Usage

```bash
Usage:
  go-tidy-check [flags] [PATH ...]

Flags:
  -d    print diffs
  -v    verbose debug output
  -version
        print version and exit
```

Within a Go workspace, it might be useful to run `go-tidy-check` for all modules of the workspace:

```bash
go-tidy-check -d $(go list -f '{{.Dir}}' -m)
```

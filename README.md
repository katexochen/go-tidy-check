[![Integration tests](https://github.com/katexochen/go-tidy-check/actions/workflows/test-integration.yml/badge.svg)](https://github.com/katexochen/go-tidy-check/actions/workflows/test-integration.yml)
[![Actionlint](https://github.com/katexochen/go-tidy-check/actions/workflows/test-lint.yml/badge.svg)](https://github.com/katexochen/go-tidy-check/actions/workflows/test-lint.yml)

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
      - uses: actions/setup-go@v4 # Go version must be at least 1.20.

      - uses: katexochen/go-tidy-check@v2
        with:
          # (Optional) The path to the root of each modules, space separated. Default is the current directory.
          modules: . ./module1 ./module2
          # (Optional) Check submodules. This will use a go.work file if present, otherwise search subdirectories
          # for go.mod files. Default is false.
          submodules: "true"
```

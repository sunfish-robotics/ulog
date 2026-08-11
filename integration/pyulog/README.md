# pyulog interoperability

This directory is an isolated Python development environment for cross-language
compatibility tests. It is not a runtime dependency of the Go package.

Run the suite from the repository root:

```console
uv sync --frozen --project integration/pyulog
ULOG_INTEROP_ARTIFACT_DIR=artifacts/pyulog \
  go test -tags=pyulog -run PyULog -count=1 .
```

The tests retain generated and rewritten `.ulg` files in the configured
artifact directory. Without that variable, Go uses a temporary directory.

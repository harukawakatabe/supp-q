# Go Server

Target commands:

```text
cmd/api
cmd/worker
cmd/admin
```

Target packages live under `internal/`.

The Go toolchain is not installed in the current local environment, so the module has not been bootstrapped or falsely marked buildable. When Go is installed, choose and document a supported version, initialize one module, add the command entrypoints, and verify `go test ./...`.


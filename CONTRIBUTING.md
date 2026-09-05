# Contributing

Small, focused pull requests are welcome. Describe the user-visible change and
how you tested it. Add regression tests for behavior fixes. Regenerate changed
Go examples rather than editing generated code by hand.

Use Go 1.26 or newer and a C compiler for race tests.

```sh
go build -o /tmp/confgen ./cmd/confgen
gofmt -w .
go vet ./...
go test ./...
go test -race ./...
go -C examples generate ./...
go -C examples test ./...
go -C examples test -race ./...
```

See [development checks](docs/development.md) for testing unpublished changes,
fuzzing, and benchmarks. Report vulnerabilities as described in [SECURITY.md](SECURITY.md).

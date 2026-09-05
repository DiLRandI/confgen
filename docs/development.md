# Development checks

Go 1.26 is the minimum because generated constraints use `new(value)`, introduced
in that release. Tests use `testing.B.Loop` and `sync.WaitGroup.Go`. CI checks
1.26 and the latest stable version, both supported under the
[Go release policy](https://go.dev/doc/devel/release).

The example pins a published revision. For unpublished changes use a temporary
workspace containing this module and `examples`. CI does this automatically.
Do not commit a local `replace` directive to the consumer example.

```sh
GOARCH=386 CGO_ENABLED=0 go test ./config ./schema ./internal/value
go test ./schema -run '^$' -fuzz '^FuzzCompile$' -fuzztime=5s -parallel=2
go test ./config -run '^$' -fuzz '^FuzzSources$' -fuzztime=5s -parallel=2
go test ./internal/value -run '^$' -fuzz '^FuzzScalarText$' -fuzztime=5s -parallel=2
go test ./config ./schema ./generator -run '^$' -bench . -benchmem
```

Generated-consumer tests compile and run an independent module. Golden examples
must match regeneration. [Benchmark results](benchmarks.md) describe their
recorded revision, not a speed guarantee.

## Repository settings

Suggested description: Type-safe Go configuration generated from YAML schemas.

Suggested topics: `golang`, `go`, `configuration`, `code-generation`, `yaml`,
`environment-variables`, `config`.

Enable private vulnerability reporting. CI and Go Reference badges are useful;
add a release badge after tagging a release. These settings remain owner-controlled.

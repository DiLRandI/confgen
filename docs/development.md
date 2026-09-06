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
go test ./infer -run '^$' -fuzz '^FuzzFromConfig$' -fuzztime=5s -parallel=2
go test ./config ./schema ./generator ./infer -run '^$' -bench . -benchmem
```

Generated-consumer tests compile and run an independent module. Golden examples
must match regeneration. [Benchmark results](benchmarks.md) describe their
recorded revision, not a speed guarantee.

## Repository settings

Suggested description: Generate type-safe Go configuration from existing config files or strict schemas.

Suggested topics: `golang`, `configuration`, `code-generation`, `yaml`, `json`,
`environment-variables`, `config`, `type-safe`.

Enable private vulnerability reporting. CI and Go Reference badges are useful;
add a release badge after tagging a release. These settings remain owner-controlled.

## Input adapters

`internal/document` owns format selection and parsing. Nodes retain ordered object
fields, array items, source positions, and scalar representations. `Scalar`
normalizes numeric representations for inference while keeping runtime raw values
unchanged. A future TOML parser should produce these same nodes; inference and Go
generation must not need format-specific branches.

`infer.FromConfig` builds the existing `schema.Model`, renders it with `schema.Render`,
and runs `schema.Compile` for validation. The generator consumes that same model
for explicit and inferred schemas. No parallel contract model is introduced.

A future dotenv adapter should provide strings, even for values such as `8080`
or `true`. `TestNormalizedTextAdapter` locks down that behavior. `TestNormalizedArrayAdapter`
checks the parser-independent array boundary. Sparse overrides select types during
inference; validation still uses the existing schema compiler. TOML and dotenv
remain deferred. Other metadata belongs in the generated full schema.

## Vulnerability checks

Install the pinned [official Go vulnerability checker](https://go.dev/doc/security/vuln/)
and scan reachable code before opening a pull request:

```sh
go install golang.org/x/vuln/cmd/govulncheck@v1.7.0
govulncheck ./...
```

CI runs the same check and fails on reachable known vulnerabilities. The tool is
installed separately and is not an application module dependency. Vulnerability
data comes from the live Go vulnerability database.

During v0.x, tests verify current generation against the current specification
and runtime. Historical generated-code compatibility coverage is deferred until
v1; regenerate consumers when upgrading.

Actions are pinned to release commit SHAs with version comments. The existing
GitHub Actions Dependabot configuration keeps update proposals enabled.

For `main`, configure a GitHub ruleset requiring a pull request and the existing
`test (1.26.x)` and `test (stable)` Go checks, blocking force pushes and branch
deletion. Optionally require branches to be up-to-date before merge. Rulesets
and private vulnerability reporting are manual repository settings.

## Test conventions

Use [Testify](https://github.com/stretchr/testify) assertions in ordinary tests:
`require` for prerequisites and `assert` for independent results. Use named
table subtests for related cases and `t.Parallel` for isolated tests. Tests
that change process environment or working directory stay serial. Assertions
in spawned goroutines must use `assert`, not `require`.

Executable Go examples retain their `Output` checks. Benchmark timed loops
retain standard error checks to avoid measuring assertion-library overhead.
Fuzz targets retain their input guards and check valid-result invariants.

`config.Source` is the application interface mocked by
[Mockery](https://vektra.github.io/mockery/latest/). Its generated implementation
lives in `config/mocks_test.go`, so mock dependencies are excluded from production
builds. Keep real file/reader adapters in integration tests.

Regenerate the mock from the repository root:

```sh
go install github.com/vektra/mockery/v3@v3.7.4
mockery
git diff --exit-code -- config/mocks_test.go
```

`.mockery.yml` selects only `config.Source`. CI repeats generation and checks
for drift. Mockery is installed separately; Testify v1.12.1 is a test dependency
in the Go module. No production package imports either library.

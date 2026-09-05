# Development and verification

The implementation covers the original v1 schema, runtime, generator, and CLI
contracts. The initial planning documents remain in Git history; this checkout
keeps the consumer schema reference and package Go documentation.

## Checks

```sh
test -z "$(gofmt -l .)"
go mod tidy
go -C examples mod tidy
go -C examples generate ./...
git diff --exit-code -- go.mod go.sum examples
go vet ./...
go -C examples vet ./...
go test ./...
go -C examples test ./...
go test -race ./...
go -C examples test -race ./...
GOARCH=386 CGO_ENABLED=0 go test ./config ./schema ./internal/value
go test ./schema -run '^$' -fuzz '^FuzzCompile$' -fuzztime=5s -parallel=2
go test ./config -run '^$' -fuzz '^FuzzSources$' -fuzztime=5s -parallel=2
go test ./internal/value -run '^$' -fuzz '^FuzzScalarText$' -fuzztime=5s -parallel=2
```

The 32-bit command targets Linux. Race and external-consumer tests need a working
C compiler. Permission tests require an unprivileged POSIX user and skip when
that condition is absent. They were executed successfully during local verification.

`generator.TestCompileGeneratedModule` generates every supported type into a
temporary independent module and runs its consumer tests under the race detector.
Checked-in generated examples also act as golden files. Update them with
`go -C examples generate ./...`, then review the diff. The example is an independent
module, so root `go test ./...` does not include it.

## Acceptance coverage

The case numbers below refer to the original 46-case acceptance contract in Git
history. They identify behavior coverage, not a percentage of code coverage.

| Cases | Behavior | Principal tests |
| --- | --- | --- |
| 1–6, 23–25, 37–40 | Files, readers, defaults, precedence, object and collection merge | `TestFilesAndReaders`, `TestPresencePrecedenceAndCollections`, `TestFinalErrors` |
| 7–10, 20, 43 | Presence and ordered error aggregation | `TestPresencePrecedenceAndCollections`, `TestFinalErrors`, `TestSyntheticPresence` |
| 11–16 | Duration and constraints | `TestScalarConversion`, `TestExactBoundsAndLengths`, `TestConstraintsAndRedaction` |
| 17–19, 29–30, 41 | Unknown fields, duplicates, nulls | `TestSourceValidationBeforeMerge`, `TestJSONErrorCategories`, `TestUnknownIgnoreAndNullOverride`, `TestEmptyObjectNull` |
| 21–22 | Provenance and secret-safe diagnostics | `TestConstraintsAndRedaction`, `TestSecretDefaultDiagnostic` |
| 26–28 | Missing files and read/permission errors | `TestFilesAndReaders`, `TestPermissionError` |
| 31–33 | Invalid defaults and name collisions | `TestValidation`, `TestRootAndCollisions`, `TestCLI` |
| 34–36 | Formatting, deterministic generation, compilation | `TestGoldenAndDeterminism`, `TestCompileGeneratedModule` |
| 42 | Cancellation and no partial result | `TestCancellation`, generated `TestGeneratedConsumer` |
| 44–46 | JSON env collections and env mappings | `TestListOfObjects`, `TestPresencePrecedenceAndCollections`, `TestEnvLoadTimeAndDisabled` |

Additional tests cover repeated/concurrent loads, independent mutable defaults,
integer overflow on 32-bit and 64-bit targets, finite floats, nested secret
templates, output preservation, and recovery backups after rollback failures.

## CI and publication

CI runs the checks above plus benchmark smoke tests. Timing thresholds are not
enforced on shared runners. Weekly Dependabot updates cover Go dependencies and
GitHub Actions. Workflow syntax is checked locally with actionlint v1.7.12.

Local verification used Go 1.27.1 on Linux amd64. Hosted checks are available in
the repository's [Actions tab](https://github.com/DiLRandI/confgen/actions).
The module path is `github.com/DiLRandI/confgen`. No version tag has been created.
Choose a license before distributing the library under an open-source license.

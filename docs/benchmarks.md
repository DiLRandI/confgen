# Performance measurements

Measured on 2026-09-05 at implementation commit `36fc45f` with Go 1.27.1,
Linux 7.0.0-30-generic, amd64, and an Intel Core i7-11800H at 2.30 GHz.
The machine has 8 cores and 16 logical CPUs. Go reported `GOMAXPROCS=16`.
Benchmarks are serial, run without the race detector, and exclude setup through
`testing.B.Loop`. No other verification commands ran concurrently with this run.

Each row reports the median of three samples. The range shows the fastest and
slowest observed time. Allocation counts were stable across the samples.

| Benchmark | Median ns/op | Time range ns/op | Median B/op | Allocs/op |
| --- | ---: | ---: | ---: | ---: |
| `LoadDefaults` | 1,934 | 1,932-1,936 | 4,784 | 26 |
| `LoadEnv` | 2,684 | 2,679-2,729 | 5,552 | 34 |
| `LoadJSON` | 7,841 | 7,706-8,160 | 10,239 | 119 |
| `LoadReader` | 19,131 | 18,786-19,392 | 21,778 | 215 |
| `LoadFile` | 23,066 | 22,796-23,359 | 22,714 | 220 |
| `Compile` | 41,491 | 40,919-41,608 | 41,505 | 531 |
| `Generate` | 584,544 | 581,254-597,636 | 125,993 | 2,287 |

## Workloads

- Runtime tests use seven leaf fields, including a nested server, duration, bool,
  string list, and string map. Defaults and env benchmarks populate fewer fields
  than the file and reader benchmarks, so their timings are not parser comparisons.
- `LoadDefaults` resolves generated-style defaults and assigns a fresh struct.
- `LoadEnv` uses an isolated map-backed lookup through `WithLookupEnv`; it measures
  textual conversion and loading, not operating-system environment lookup costs.
- `LoadJSON` parses a snapshotted JSON reader on each load.
- `LoadReader` parses equivalent YAML from a snapshotted reader on each load.
- `LoadFile` rereads a temporary YAML file on each load. Filesystem caches are warm;
  this is not a cold-disk latency measurement.
- `Compile` parses and validates a three-field schema with defaults and constraints.
- `Generate` renders and formats the repository's representative example schema
  from an already validated model. It excludes schema compilation and file writes.

These are startup configuration workloads. They do not establish performance
limits for large inputs, concurrent loads, or other machines. CI runs benchmarks
as smoke checks without enforcing timing thresholds on shared runners.

## Reproduce

```sh
go version
go test ./config ./schema ./generator -run '^$' -bench . -benchmem -benchtime=500ms -count=3
```

For comparisons, use the same machine and Go version, keep unrelated workloads
idle, and collect more samples with `-count=10`. The benchmark sources are
`config/benchmark_test.go`, `config/load_test.go`, `schema/example_test.go`, and
`generator/generate_test.go`.

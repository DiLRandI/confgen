# Runtime Specification

## 1. Goal

The runtime package loads configuration into a generated Go type while preserving a minimal application-facing API and deterministic behavior.

Normal use:

```go
cfg, err := appconfig.Load(
    config.File("config.yaml"),
    config.Env(),
)
```

Fail-fast use:

```go
cfg := appconfig.MustLoad(
    config.OptionalFile("config.yaml"),
    config.Env(),
)
```

## 2. Public API target

```go
type Source interface {
    Name() string
    Load(ctx context.Context, descriptor *Descriptor) (Document, error)
}

func File(path string) Source
func OptionalFile(path string) Source
func Reader(name string, r io.Reader, format Format) Source
func Env(options ...EnvOption) Source

type Format uint8

const (
    FormatYAML Format = iota + 1
    FormatJSON
)

func Load[T any](
    ctx context.Context,
    descriptor Descriptor,
    sources ...Source,
) (*T, error)
```

Most users call generated wrappers, not `config.Load[T]` directly.

## 3. Generated wrappers

Generated package API:

```go
func Load(sources ...config.Source) (*Config, error)
func LoadContext(ctx context.Context, sources ...config.Source) (*Config, error)
func MustLoad(sources ...config.Source) *Config
```

`Load` uses `context.Background()`. `MustLoad` panics with the returned error when loading fails.

## 4. Precedence

Sources are evaluated left to right. Later sources win.

```go
appconfig.Load(
    config.File("config.yaml"),
    config.Env(),
)
```

means:

```text
defaults < file < environment
```

Reverse source order reverses precedence.

## 5. Presence semantics

Each field tracks whether a value was supplied independently from the value itself.

Valid present values include:

- integer `0`;
- boolean `false`;
- empty string `""`;
- empty list `[]`;
- empty map `{}`.

`required` checks presence, never zero-value equality.

## 6. Defaults

Generated defaults seed the effective state before any source runs.

A default has provenance similar to `default:<path>` internally and counts as present.

## 7. File source

`File(path)`:

- reads when `Load` executes;
- requires the file to exist;
- supports `.yaml`, `.yml`, `.json`;
- infers format from extension;
- reports unsupported extensions;
- reports read/permission errors;
- checks YAML/JSON syntax;
- checks duplicate keys;
- checks unknown fields according to descriptor policy.

`OptionalFile(path)` behaves identically except `os.IsNotExist` contributes an empty document rather than an error. Other filesystem errors remain fatal.

## 8. Reader source

```go
config.Reader("embedded", r, config.FormatYAML)
```

Reader sources require explicit format and a diagnostic source name.

The implementation must define whether a `Reader` is single-use. Recommended v1 behavior: read the entire reader at source construction time into immutable bytes, so repeated `Load` calls are deterministic. Document this difference from `File`, which is reread each time.

## 9. Environment source

`Env()` reads environment variables when `Load` executes.

Use `os.LookupEnv`, not `os.Getenv`, so an explicitly empty environment variable is present.

The source only queries environment variable names known by the descriptor. Unrelated process environment variables are ignored.

## 10. Environment scalar conversion

Environment variables are strings and convert textually:

- `string`, `path`: exact string;
- `bool`: `strconv.ParseBool`;
- `int`: `strconv.ParseInt` with platform-width check before assignment;
- `int64`: `strconv.ParseInt(..., 10, 64)`;
- `uint`: `strconv.ParseUint` with platform-width check;
- `uint64`: `strconv.ParseUint(..., 10, 64)`;
- `float64`: `strconv.ParseFloat(..., 64)`;
- `duration`: `time.ParseDuration`.

Whitespace is not silently trimmed unless the underlying standard parser does so. Configuration should be explicit.

## 11. Environment lists and maps

Collections use JSON syntax, not comma splitting.

```text
MYAPP_ALLOWED_ORIGINS=["https://a.example","https://b.example"]
MYAPP_LABELS={"environment":"production","region":"eu"}
```

Lists of objects also use JSON syntax.

## 12. File type rules

YAML/JSON files are strongly typed. Do not broadly coerce quoted numbers or booleans into numeric/bool fields.

Examples for an `int` field:

```yaml
port: 8080      # valid
port: "8080"    # type error if effective
```

Exceptions are schema types naturally represented as strings, including `duration` and `path`.

## 13. Unknown fields

Default descriptor policy is `error`.

Unknown YAML/JSON paths produce `IssueUnknownField` even if a later source would otherwise override related values.

With `unknown_fields: ignore`, unknown file fields are ignored.

Environment source does not scan arbitrary environment keys and therefore does not produce unknown-env errors.

## 14. Duplicate keys

Duplicate object keys are errors in YAML and JSON.

YAML must be checked through node traversal because ordinary decode may overwrite duplicates.

JSON must use token-level object parsing or another approach that detects duplicate names before ordinary map collapse.

## 15. Merge semantics

Objects merge by leaf field.

Lists replace atomically.

Maps replace atomically; no deep map merge in v1.

A source value only overrides a field when that field is present in the source.

## 16. Null

`null` is not a valid effective v1 field value.

Because conversion happens after merge, a lower-priority null can be superseded by a higher-priority non-null value. An effective null yields a type/unsupported-null issue.

## 17. Conversion after merge

Only effective final values are converted.

This is intentional:

```yaml
# lower-priority file
port: not-an-int
```

plus:

```text
MYAPP_PORT=8080
```

may load successfully when env has higher precedence.

Syntax, duplicate-key, and unknown-field errors are source-level and cannot be hidden by later sources.

## 18. Validation order

After merge:

1. convert effective values;
2. required validation;
3. type-specific constraints;
4. populate final Go struct.

Aggregate independent final issues where practical.

## 19. Required validation

Missing means no default and no source supplied the field.

`0`, `false`, and `""` are present and can satisfy required.

## 20. Constraints

Supported runtime checks:

- numeric/duration `min` and `max`;
- string/list/map `min_length` and `max_length`;
- scalar `enum`;
- string/path regex `pattern`.

Patterns are prevalidated by the generator; runtime may store precompiled metadata or compile once from generated constants during package initialization.

## 21. Secret redaction

Any issue attached to a `secret` field must replace user-provided value text with `[REDACTED]` in `Error()` output.

Structured error objects should also avoid exposing raw secret values. Prefer not storing raw secret values in `Issue` at all.

## 22. Source provenance

Every effective value should retain the source that supplied it. Suggested labels:

```text
default:server.port
file:config.yaml
env:MYAPP_SERVER_PORT
reader:embedded
```

File source locations should additionally preserve line/column when available.

## 23. No partial success

If any fatal source, conversion, required, or constraint issue exists, return `nil, error`. Do not return a partially populated config alongside an error.

## 24. Context

Check `ctx.Err()` before each source and after potentially blocking source operations. Custom sources receive the context directly.

## 25. Panic behavior

`MustLoad` is the only convenience API that panics. Runtime internals must not intentionally panic for user configuration errors.

Generator bugs or programmer misuse may still panic only where ordinary Go conventions make that unavoidable, but public APIs should prefer errors.

## 26. Thread safety

Runtime loading has no mutable global state. Concurrent `Load` calls are safe when all custom source implementations are safe.

## 27. Suggested internal pipeline

```text
Descriptor defaults
  -> effective map
  -> Source.Load()
  -> source-level validation
  -> normalize canonical paths
  -> overwrite effective map
  -> repeat
  -> convert effective leaves
  -> final validation
  -> allocate T
  -> assign fields
  -> return *T
```

## 28. Extensibility boundary

Third-party `Source` implementations may fetch from remote systems later. They must return raw values and provenance and leave merge/conversion/validation to the core runtime.

Remote providers are not shipped in v1.

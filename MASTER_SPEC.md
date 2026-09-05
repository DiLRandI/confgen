# Master Product Specification — v1

## Product statement

A schema-first Go configuration generator and runtime. Developers define configuration once, generate idiomatic Go types, and load YAML, JSON, and environment values through a tiny ordered-source API.

## Ideal user workflow

1. Author `config.schema.yaml`.
2. Add a `//go:generate` directive invoking `configgen`.
3. Run `go generate ./...`.
4. Commit generated `config_gen.go` if project policy prefers committed generated code.
5. Load at application startup:

```go
cfg := appconfig.MustLoad(
    config.OptionalFile("config.yaml"),
    config.Env(),
)
```

## Example schema

```yaml
version: 1
package: appconfig
name: Config
env_prefix: SHOP
unknown_fields: error

fields:
  server:
    type: object
    fields:
      host:
        type: string
        default: "0.0.0.0"
      port:
        type: int
        default: 8080
        min: 1
        max: 65535
      timeout:
        type: duration
        default: 30s

  database:
    type: object
    fields:
      url:
        type: string
        required: true
        secret: true
        env: DATABASE_URL

  storage_dir:
    type: path
    default: "./data"

  debug:
    type: bool
    default: false

  log_level:
    type: string
    default: info
    enum: [debug, info, warn, error]
```

## Generated application-facing shape

Conceptually:

```go
type Config struct {
    Server     ServerConfig `json:"server" yaml:"server"`
    Database   DatabaseConfig `json:"database" yaml:"database"`
    StorageDir string `json:"storage_dir" yaml:"storage_dir"`
    Debug      bool `json:"debug" yaml:"debug"`
    LogLevel   string `json:"log_level" yaml:"log_level"`
}

type ServerConfig struct {
    Host    string
    Port    int
    Timeout time.Duration
}

type DatabaseConfig struct {
    URL string
}
```

Generated package also exports:

```go
func Load(sources ...config.Source) (*Config, error)
func LoadContext(ctx context.Context, sources ...config.Source) (*Config, error)
func MustLoad(sources ...config.Source) *Config
```

## Runtime semantics

### Precedence

```text
defaults < source 1 < source 2 < ...
```

Later source wins.

### Presence

`0`, `false`, and `""` are valid present values. `required` is presence-based.

### Object merge

Merge object leaves independently.

### Collection merge

Replace lists and maps atomically.

### Type conversion

Resolve precedence first, then convert effective values. Syntax errors, unknown fields, and duplicate keys are still reported even for lower-priority sources.

### Environment

Use descriptor-known names only and `os.LookupEnv` semantics. Collections are JSON strings. Automatic nested naming uses uppercase underscore paths with optional root prefix.

### Files

YAML and JSON only. Required by default; optional file is explicit. Unknown fields error by default. Duplicate object keys error.

## Type system

v1:

```text
string
bool
int
int64
uint
uint64
float64
duration
path
object
list
map[string]T
```

`duration` uses Go duration syntax. `path` generates `string` in v1.

## Validation

v1 constraints:

- `required`;
- `default`;
- `min` / `max`;
- `min_length` / `max_length`;
- `enum`;
- `pattern`;
- `secret` metadata.

## Error contract

Return structured `*config.Error` with ordered `Issue` entries where practical. Include canonical path and source provenance. Preserve file line/column when possible. Never render raw secret values.

## v1 non-goals

No flags, TOML, `.env`, remote provider implementations, hot reload, templating, pointer/null support, dynamic getters, or UI.

## Acceptance condition

The library is v1-ready when one schema can generate compiling Go code that successfully loads ordered YAML/JSON/env sources, validates them according to this contract, and passes the complete `TEST_PLAN.md` suite.

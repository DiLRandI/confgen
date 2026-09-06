# confgen

Define config once. Generate type-safe Go.

confgen generates typed Go configuration from existing YAML/JSON files or strict
YAML schemas, with defaults, validation, and environment overrides. Requires Go 1.26 or newer.

## Why confgen?

Keep configuration fields and their rules together. Generate Go structs instead
of maintaining them by hand, then load configuration with one call.

## Quick start

### Install

Inside your application's Go module:

```sh
go get github.com/DiLRandI/confgen/config@latest
go install github.com/DiLRandI/confgen/cmd/confgen@latest
```

Make sure your Go bin directory is on `PATH`. No checkout is needed.

### Already have config.yaml?

```sh
confgen init --from config.yaml --package appconfig
```

This infers structure and types, creating `appconfig/config.schema.yaml` and
`appconfig/config_gen.go`. It leaves your file untouched and refuses to replace
existing outputs. Runtime values are **not copied** into generated source unless
you explicitly pass `--copy-defaults`.

Keys such as `server-port`, `databaseURL`, and `logging.level` keep working. The
schema records their original spelling with `key`; generated Go uses exported names.
Load the original file with `appconfig.Load(config.OptionalFile("config.yaml"), config.Env())`.

For nulls, empty lists, or explicit types, create `confgen.overrides.yaml`:

```yaml
fields:
  database_url: {type: string}
  allowed_hosts: {type: list, items: {type: string}}
```

```sh
confgen init --from config.yaml --package appconfig --overrides confgen.overrides.yaml
```

Overrides use canonical names. See the [migration guide](docs/migration.md) for a
complete example. After onboarding, edit the schema and use `confgen generate`.
Do not rerun `init` as part of normal generation.

### Prefer a schema first?

Create `appconfig/config.schema.yaml`:

```yaml
version: 1
package: appconfig
env_prefix: APP
fields:
  port:
    type: int
    default: 8080
    min: 1
    max: 65535
  debug:
    type: bool
    default: false
```

### Generate

```sh
confgen generate -schema appconfig/config.schema.yaml -out appconfig/config_gen.go
```

For repeatable generation, add `appconfig/generate.go`:

```go
package appconfig

//go:generate go run github.com/DiLRandI/confgen/cmd/confgen generate -schema config.schema.yaml -out config_gen.go
```

### Load

Save as `main.go`, replacing `example.com/myapp` with your module path:

```go
package main

import (
    "fmt"
    "log"

    "example.com/myapp/appconfig"
    "github.com/DiLRandI/confgen/config"
)

func main() {
    cfg, err := appconfig.Load(config.OptionalFile("config.yaml"), config.Env())
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(cfg.Port, cfg.Debug)
}
```

With the schema-first example above, run `APP_PORT=9000 go run .` to print `9000 false`.

## Configuration precedence

Defaults → sources from left to right. Later sources win.
Above: schema defaults → `config.yaml` → environment.
Objects merge by field; lists and maps replace as a whole.

## Features

- Typed scalars, nested objects, lists, and string-keyed maps.
- YAML and JSON files, readers, and environment variables.
- Defaults, required fields, validation, and secret-safe diagnostics.
- Deterministic Go generation and optional configuration templates.
- Editable schema bootstrapping from existing YAML or JSON.

## Schema overview

Each field has a `type` and optional metadata. Supported types include `string`,
`bool`, `int`, `int64`, `uint`, `uint64`, `float64`, `duration`, `path`, `object`,
`list`, and `map`. See the [schema reference](docs/schema.md).

## Environment variables

`env_prefix: APP` maps `server.port` to `APP_SERVER_PORT`. Use `env: PORT` for an
explicit name or `env: false` to disable a mapping. Lists and maps use JSON.

## Validation

Use `required`, `min`/`max`, `min_length`/`max_length`, `enum`, and `pattern`.
Required means present: `0`, `false`, and `""` count. Unknown file fields and
duplicate keys are errors by default. Loads return no partial config on failure.

## Secrets

`secret: true` protects library diagnostics, not application logging of the
returned struct. Never put credentials in schema defaults.

## Documentation

[Go reference](https://pkg.go.dev/github.com/DiLRandI/confgen/config) ·
[Migration guide](docs/migration.md) ·
[Application example](examples/README.md) · [Advanced loading](docs/runtime.md) ·
[Benchmarks](docs/benchmarks.md)

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for setup and checks.

## License

[MIT](LICENSE).

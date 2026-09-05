# confgen

Define configuration once in YAML, generate Go structs, and load YAML, JSON,
and environment variables. Later sources win. Defaults, validation, and secret
metadata come from the schema; runtime code never reads the schema file.

## Install

Requires Go 1.27.1 or newer. From this checkout:

```sh
go install ./cmd/configgen
```

The module path is `github.com/DiLRandI/confgen`. Once published, consumers can use
`go get github.com/DiLRandI/confgen/config` and install the generator with
`go install github.com/DiLRandI/confgen/cmd/configgen@latest`.

For a working local consumer today, see the [separate example module](examples/README.md).
Its `go.mod` replaces the dependency with this checkout.

## Define and generate

Save this as `appconfig/config.schema.yaml`:

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
  database_url:
    type: string
    required: true
    secret: true
```

Add `appconfig/generate.go`:

```go
package appconfig

//go:generate go run github.com/DiLRandI/confgen/cmd/configgen -schema config.schema.yaml -out config_gen.go
```

Run `go generate ./...`, then `go mod tidy`. Commit the generated Go file.
The CLI also accepts `-example-yaml config.example.yaml` and
`-example-env .env.example`. These are templates; the library does not load
`.env` files. `-runtime-import` overrides the generated runtime import path.

## Load

Import your generated `appconfig` package and `github.com/DiLRandI/confgen/config`, then:

```go
cfg, err := appconfig.Load(
    config.OptionalFile("config.yaml"),
    config.Env(),
)
if err != nil {
    return err
}
fmt.Println(cfg.Port)
```

Set `APP_DATABASE_URL` before loading. `APP_PORT` overrides the file and default.
Generated packages also provide `LoadContext` and `MustLoad`.

Required means present: `0`, `false`, and `""` count. Objects merge by leaf;
lists and maps replace. Unknown file fields and duplicate keys are errors.
Secret diagnostics are redacted, but printing the config struct can expose
secrets. Never put real credentials in schema defaults.

See the [schema reference](docs/schema.md), [executable examples](examples/appconfig/example_test.go),
[benchmark results](docs/benchmarks.md), and [development checks](docs/development.md).
Read API documentation with `go doc -all ./config`, `go doc -all ./schema`,
or `go doc -all ./generator`. Run the example tests with `go -C examples test -v ./...`.

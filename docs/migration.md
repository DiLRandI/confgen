# Start from existing configuration

You do not need to rename existing keys or recreate your configuration by hand.
Given `config.yaml`:

```yaml
server-port: 8080
databaseURL: null
allowed-hosts: []
```

Null and empty-list values do not provide enough type information. Create
`confgen.overrides.yaml` using canonical names:

```yaml
fields:
  database_url: {type: string}
  allowed_hosts: {type: list, items: {type: string}}
```

Bootstrap once:

```sh
confgen init --from config.yaml --package appconfig --overrides confgen.overrides.yaml
```

This creates `appconfig/config_gen.go` and an editable `appconfig/config.schema.yaml`:

```yaml
version: 1
package: appconfig
fields:
  server_port:
    type: int64
    key: server-port
  database_url:
    type: string
    key: databaseURL
  allowed_hosts:
    type: list
    key: allowed-hosts
    items:
      type: string
```

Neither input file is modified. Runtime values are not copied into the schema
or generated Go. Add `--copy-defaults` only when you intend to embed those values;
review them for credentials before committing. Nulls are never copied as defaults.

## Load the original file

Use the generated package with the runtime library:

```go
cfg, err := appconfig.Load(config.File("config.yaml"), config.Env())
```

In this example, set `DATABASE_URL` in the environment to override the null value.
Runtime nulls remain errors if no later source replaces them. `cfg.ServerPort`
comes from `server-port`, and `cfg.AllowedHosts` is an empty string slice.

## Own the schema from here

Add descriptions, required fields, secrets, defaults, environment mappings, and
validation rules to the full schema. Then regenerate:

```sh
confgen generate --schema appconfig/config.schema.yaml --out appconfig/config_gen.go
```

`init` is one-time onboarding. `generate` is the ongoing schema-driven workflow
and preserves your added metadata. Never put `init` in a recurring `go:generate`.
See the README for the generation directive. Incompatible generated type changes
show up as compilation errors in callers.

`init` creates missing directories and refuses existing destinations. Explicit
`--schema` and `--out` paths each override their package-relative default. Use new
destinations to try another inference; there is no overwrite flag.

## Inference rules

| Input | Inferred type |
| --- | --- |
| Mapping | Object, in declaration order |
| String, including `30s` | String |
| Boolean | Bool |
| Signed-range integer | Int64 |
| Larger positive integer fitting 64 bits | Uint64 |
| Finite floating-point value | Float64 |
| Compatible non-empty list | List |
| Empty object | Empty object |

Object lists use the union of their fields in first-seen order. Missing fields
are optional; conflicting types still fail. Nulls and empty lists require type
overrides or an explicit schema. Nested lists and out-of-range numbers are errors.
Quote YAML dates to keep them strings. Aliases, merge keys, and duplicate keys
are rejected. Input extensions are `.yaml`, `.yml`, and `.json`.

Inference never guesses durations, paths, maps, secrets, required fields,
constraints, or descriptions. Numeric-looking strings stay strings.

## Key names

The schema's `key` property preserves the original file key. `server-port`,
`serverPort`, and `ServerPort` normalize to `server_port`; `HTTPServer` becomes
`http_server`, and `logging.level` becomes `logging_level`. Environment names use
the canonical path, so `server_port` maps to `SERVER_PORT` unless a prefix or
explicit environment name is set.

Existing snake_case names stay unchanged. Separators become underscores, leading
digits get a `field_` prefix, and non-ASCII characters use `u` plus their hexadecimal
code point. A punctuation-only name becomes `field`. Empty keys and keys that
cannot fit JSON/YAML struct tags are rejected. See [external keys](schema.md#external-keys).

Two keys mapping to the same canonical name are an error, including across list
items. confgen does not silently merge them or invent numeric suffixes.

## Override scope

Overrides support `type`, scalar `items.type`, and scalar `values.type` only:

```yaml
fields:
  server.timeout: {type: duration}
  labels: {type: map, values: {type: string}}
```

Use canonical dotted paths for nested object fields. Unknown paths, incompatible
values, and unsupported properties fail. Overrides inside list objects and new
object shapes are not supported; edit the full schema for those cases.

With `--copy-defaults`, only compatible non-null values become defaults. A
collection containing null is omitted as a whole rather than partially copied.

For direct generation without saving a schema, use the same inference options:

```sh
confgen generate --from config.json --package appconfig --out appconfig/config_gen.go
```

Prefer `init` when you want to maintain and enrich a configuration contract.

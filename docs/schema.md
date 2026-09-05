# Schema reference

## 1. Overview

The schema is the authoritative configuration contract.

Recommended filename:

```text
config.schema.yaml
```

v1 schemas are YAML only and contain exactly one YAML document.

Every schema begins with:

```yaml
version: 1
```

Unsupported versions are rejected.

## 2. Root grammar

```yaml
version: 1
package: appconfig
name: Config
env_prefix: MYAPP
unknown_fields: error
fields:
  server:
    type: object
    fields:
      port:
        type: int
        default: 8080
```

Supported root properties:

| Property | Required | Meaning |
|---|---:|---|
| `version` | yes | Must equal `1`. |
| `package` | yes | Generated Go package identifier. |
| `name` | no | Root generated Go type; defaults to `Config`. |
| `env_prefix` | no | Prefix for automatically generated env names. |
| `unknown_fields` | no | `error` or `ignore`; defaults to `error`. |
| `fields` | yes | Root field mapping. |

Unknown root properties are schema errors.

## 3. Field-key syntax

Schema field keys must match:

```text
[a-z][a-z0-9_]*
```

Valid: `port`, `server_port`, `log_level`, `api_url`.

Invalid: `server-port`, `ServerPort`, `server.port`, `123port`.

Field order in generated Go and generated examples follows schema order.

## 4. Supported types

| Schema type | Generated Go type |
|---|---|
| `string` | `string` |
| `bool` | `bool` |
| `int` | `int` |
| `int64` | `int64` |
| `uint` | `uint` |
| `uint64` | `uint64` |
| `float64` | `float64` |
| `duration` | `time.Duration` |
| `path` | `string` |
| `object` | generated named struct |
| `list` | `[]T` |
| `map` | `map[string]T` |

Unknown types are schema errors.

## 5. Common field properties

Depending on type, fields may support:

```yaml
type: string
go_name: APIEndpoint
description: Public API endpoint.
required: true
default: example
env: EXPLICIT_ENV_NAME
secret: false
enum: [one, two]
min: 1
max: 10
min_length: 1
max_length: 20
pattern: "^[a-z]+$"
```

Properties that do not make sense for a field type are schema errors; they are never silently ignored.

## 6. Objects

```yaml
server:
  type: object
  fields:
    host:
      type: string
    port:
      type: int
```

`fields` is required. `required` is not supported directly on object fields in v1. Requiredness belongs on leaves.

Generated code should use deterministic named nested types, e.g. `ServerConfig`.

## 7. Lists

```yaml
allowed_origins:
  type: list
  items:
    type: string
```

`items` is required. Lists may contain scalar types, durations, paths, or objects.

Example list of objects:

```yaml
backends:
  type: list
  items:
    type: object
    fields:
      name:
        type: string
        required: true
      url:
        type: string
        required: true
```

Lists are atomic merge values.

## 8. Maps

```yaml
labels:
  type: map
  values:
    type: string
```

Map keys are always strings in v1. `values` is required.

v1 map values may be scalar, duration, or path types. Maps of objects are deferred. Maps are atomic merge values.

## 9. Go-name generation

Schema names map deterministically to exported Go identifiers:

```text
server       -> Server
server_port  -> ServerPort
log_level    -> LogLevel
api_url      -> APIURL
http_port    -> HTTPPort
database_id  -> DatabaseID
tls_enabled  -> TLSEnabled
```

Use a fixed initialism set that includes at least:

`API`, `ASCII`, `CPU`, `CSS`, `DNS`, `EOF`, `GUID`, `HTML`, `HTTP`, `HTTPS`, `ID`, `IP`, `JSON`, `QPS`, `RAM`, `RPC`, `SLA`, `SMTP`, `SQL`, `SSH`, `TCP`, `TLS`, `TTL`, `UDP`, `UI`, `UID`, `URI`, `URL`, `UTF8`, `UUID`, `VM`, `XML`, `XMPP`, `XSRF`, `XSS`.

A field may override its generated name:

```yaml
api_url:
  type: string
  go_name: APIEndpoint
```

`go_name` must be a valid exported Go identifier. Duplicate generated names within a struct are schema errors.

## 10. Canonical field paths

Canonical paths use schema keys, not generated Go names.

```text
server.port
database.dsn
log_level
```

Canonical paths are immutable within a schema version and are used by runtime metadata, errors, merging, and env-name generation.

## 11. Environment-name generation

Given `env_prefix: MYAPP`, canonical path `server.timeout` becomes:

```text
MYAPP_SERVER_TIMEOUT
```

Algorithm:

1. split path by `.`;
2. uppercase every component without changing underscores;
3. join with `_`;
4. prepend `<ENV_PREFIX>_` when a prefix exists.

Examples:

```text
server.host     -> MYAPP_SERVER_HOST
server.timeout  -> MYAPP_SERVER_TIMEOUT
log_level       -> MYAPP_LOG_LEVEL
```

A field can override the generated variable:

```yaml
dsn:
  type: string
  env: DATABASE_URL
```

A field can disable environment mapping:

```yaml
internal_name:
  type: string
  env: false
```

All enabled environment names must be unique across the full schema. Duplicate mappings are schema errors.

## 12. Required

```yaml
database_url:
  type: string
  required: true
```

`required` means a value must be present after defaults and runtime source merging.

Presence is independent of zero value, therefore `0`, `false`, and `""` can satisfy `required`.

A valid default establishes presence and satisfies `required`.

## 13. Defaults

Defaults use the field's schema representation and must be validated by `configgen`.

Examples:

```yaml
port:
  type: int
  default: 8080

timeout:
  type: duration
  default: 30s

debug:
  type: bool
  default: false
```

An invalid default is a generation error.

Object defaults are not supported in v1. Defaults for lists and maps are permitted only when the generator can validate every contained value against the declared item/value type.

## 14. Duration

`duration` maps to `time.Duration` and accepts exactly Go `time.ParseDuration` syntax.

Valid: `500ms`, `30s`, `5m`, `1h30m`, `1.5h`.

`2d` is invalid in v1; use `48h`.

Duration values in YAML/JSON are represented as strings.

## 15. Path

`path` maps to Go `string`. v1 does not check existence, file-vs-directory, or absolute-vs-relative semantics. Keeping a distinct schema kind reserves room for later path-specific validation.

## 16. Numeric constraints

`min` and `max` are inclusive.

```yaml
port:
  type: int
  min: 1
  max: 65535
```

Supported on numeric types and `duration`. Duration constraints use duration strings.

Schema validation rejects `min > max`.

## 17. Length constraints

`min_length` and `max_length` apply to strings, lists, and maps.

They are integer counts, inclusive, and must be non-negative. Schema validation rejects `min_length > max_length`.

## 18. Enum

```yaml
log_level:
  type: string
  enum: [debug, info, warn, error]
```

v1 supports enum constraints for scalar types. Enum entries must match the field type and be unique.

Defaults must satisfy the enum.

## 19. Pattern

`pattern` is supported for `string` and `path` only and uses Go `regexp` syntax.

Patterns are compiled during generation. Invalid regular expressions are schema errors.

## 20. Secret

```yaml
password:
  type: string
  required: true
  secret: true
```

`secret` may be attached to scalar, list, or map leaf fields. It affects library-generated diagnostics only. It does not change the generated Go type.

## 21. Nullability

YAML/JSON `null` is unsupported as an effective field value in v1. Pointer/nullable generation is out of scope.

A null value in a lower-priority source may be overridden by a higher-priority non-null value because final conversion occurs after merge; nevertheless the source must remain syntactically valid and schema-known.

## 22. Unknown schema properties

Unknown properties at root, field, item, or map-value level are schema errors. This prevents typo-driven silent behavior.

## 23. Duplicate keys

Duplicate mapping keys in the schema are errors, including duplicates hidden by YAML parsing behavior. The parser must inspect YAML nodes sufficiently early to detect duplicates.

## 24. Property compatibility matrix

At minimum enforce:

| Property | Types |
|---|---|
| `required` | all leaf types |
| `default` | all leaf types except unsupported composite cases |
| `secret` | all leaf types |
| `env` | all leaf types |
| `enum` | scalar types |
| `min`, `max` | numeric types, duration |
| `min_length`, `max_length` | string, path, list, map |
| `pattern` | string, path |
| `fields` | object only |
| `items` | list only |
| `values` | map only |

## 25. Schema validation ordering

Recommended ordering:

1. YAML syntax and duplicate-key check.
2. root-property validation.
3. field-key validation.
4. type validation.
5. property/type compatibility.
6. recursive child validation.
7. generated Go-name collision detection.
8. canonical path generation.
9. env-name generation and collision detection.
10. default validation.
11. constraint validation.

Collect multiple independent issues where practical rather than stopping at the first semantic error.

## Implementation notes

`schema.Compile` parses and validates the complete schema. `schema.Parse` alone
checks structure and retains YAML nodes; it does not validate defaults or types.
Diagnostics currently stop at the first schema error. Runtime final validation
aggregates independent failures in schema order.

The generated package must have a non-main Go package name and an exported root
type name. Root and nested type names must not collide with generated helpers.
Empty object schemas are supported.

String lengths count Unicode code points. Floats must be finite. YAML aliases,
merge keys, and non-string mapping keys are rejected. Collection item fields
cannot have independent environment mappings; set the whole list or map with
its parent environment variable. Explicit environment names use portable
`[A-Za-z_][A-Za-z0-9_]*` syntax.

Never store real credentials in schema defaults. Defaults appear in generated
Go source. Generated YAML and environment examples replace secret values with
neutral placeholders, including secrets inside list-of-object defaults.

# Schema reference

A schema is one YAML document with `version: 1`, a Go `package`, and `fields`.
Optional root properties are `name`, defaulting to `Config`, `env_prefix`, and
`unknown_fields`, which is `error` by default or `ignore`.

Field names use lowercase letters, digits, and underscores, starting with a letter.
`go_name` overrides the exported Go name. `description` becomes field GoDoc.
Duplicate keys, unknown properties, and generated name collisions are errors.

## External keys

Use `key` when the file key differs from the canonical schema name:

```yaml
fields:
  server_port: {type: int, key: server-port}
```

This reads `server-port` from YAML/JSON and generates `ServerPort` with matching
struct tags. Paths and automatic environment mappings still use `server_port`,
so `env_prefix: APP` produces `APP_SERVER_PORT`. The same rule applies to nested
objects and list members. Object keys inside collection defaults use external names.

External keys must be unique within their object and non-empty. They must also
fit JSON and YAML struct tags: commas, quotes, backslashes, control characters,
backticks, and the special key `-` are rejected. Common camelCase, PascalCase,
kebab-case, dotted names, and Unicode letters are supported. Omit `key` to use
the canonical name. Collection `items` and `values` cannot have their own key.

## Types

| Schema type | Go type |
| --- | --- |
| `string`, `path` | `string` |
| `bool` | `bool` |
| `int`, `int64`, `uint`, `uint64`, `float64` | Same Go type |
| `duration` | `time.Duration` |
| `object` | Named struct |
| `list` | `[]T` |
| `map` | `map[string]T` |

Durations use Go syntax such as `500ms` or `1h30m`; `2d` is unsupported.
Paths are strings without filesystem validation. Floats must be finite.
Effective null values are unsupported.

## Defaults and required fields

```yaml
version: 1
package: appconfig
fields:
  port: {type: int, default: 8080}
  database_url: {type: string, required: true, secret: true}
```

Defaults have the lowest priority and satisfy `required`. Zero, false, and empty
strings count as present. Defaults must satisfy the field's type and constraints.

## Objects, lists, and maps

```yaml
fields:
  server:
    type: object
    fields:
      host: {type: string, default: localhost}
      port: {type: int, default: 8080}
  origins:
    type: list
    items: {type: string}
  labels:
    type: map
    values: {type: string}
```

Objects merge by leaf and support empty `fields: {}`. Put `required` and defaults
on their children. Lists contain scalars or objects. Map values are scalar.
Lists and maps replace atomically and may have defaults.

## Environment variables

`env_prefix: APP` maps `server.host` to `APP_SERVER_HOST`.
Set `env: CUSTOM_NAME` to override it, or `env: false` to disable it.
Enabled names must be unique and use letters, digits, and underscores.
Collection members do not have independent env mappings; supply JSON for the parent.

## Validation

| Property | Applies to |
| --- | --- |
| `min`, `max` | Numbers and durations; inclusive |
| `min_length`, `max_length` | Strings, paths, lists, maps; inclusive |
| `enum` | Scalars; entries must be unique and correctly typed |
| `pattern` | Strings and paths; Go regular expressions |

String lengths count Unicode code points. Duration bounds are duration strings.

## Secrets

Use `secret: true` on a scalar, list, or map to mark sensitive configuration.
Library diagnostics omit raw values; templates use neutral secret placeholders.
Generated Go contains defaults, so never store actual credentials in a schema.
Printing or marshaling config structs can still expose secrets.

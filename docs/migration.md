# Start from existing configuration

Without generation, you maintain both Go fields and configuration data:

```go
type Config struct {
    Server struct { Port int }
}
```

```yaml
server:
  port: 8080
  timeout: 30s
```

Bootstrap once:

```sh
confgen init --from config.yaml --package appconfig \
  --schema appconfig/config.schema.yaml --out appconfig/config_gen.go
```

confgen creates missing output directories, an editable schema, and typed Go.
It never modifies `config.yaml` and refuses to replace existing outputs. Input
values are not copied as defaults unless you add `--copy-defaults`:

```sh
confgen init --from config.yaml --copy-defaults --package appconfig \
  --schema appconfig/config.schema.yaml --out appconfig/config_gen.go
```

Review copied values for credentials before committing.

The inferred schema uses `int64` for port and `string` for timeout. You can edit it:

```yaml
timeout:
  type: duration
  default: 30s
  min: 100ms
  description: Maximum request processing time.
```

From then on, regenerate from the schema:

```sh
confgen generate --schema appconfig/config.schema.yaml --out appconfig/config_gen.go
```

This preserves metadata you added. Put that command in a `go:generate` directive,
as shown in the README. Do not put `init` in a recurring generation command.
Changes to generated field types become visible as Go compilation errors in callers.

## Inference rules

| Input | Inferred type |
| --- | --- |
| Mapping | Object; declaration order is retained |
| String, including `30s` | String |
| Boolean | Bool |
| Signed-range integer | Int64, independent of host architecture or current magnitude |
| Larger positive integer fitting 64 bits | Uint64 |
| Finite floating-point value | Float64 |
| Compatible non-empty list | List; object item fields are merged in first-seen order and matching fields must keep the same types |
| Empty object | Empty object |

Nulls, empty lists, mixed lists, nested lists, and out-of-range numbers need an
explicit schema or input correction. Object field order may differ between list
items; new fields are appended in first-seen order. Dates with YAML timestamp tags
must be quoted to infer strings. Aliases, merge keys, and duplicate keys are rejected.

confgen never guesses duration/path semantics, maps, secrets, required fields,
constraints, or descriptions. Strings that look numeric remain strings.
Supported input extensions are `.yaml`, `.yml`, and `.json`.

For a quick disposable generation without saving a schema:

```sh
confgen generate --from config.json --package appconfig --out appconfig/config_gen.go
```

Add `--copy-defaults` to this command when the input values should become
defaults in the generated contract.

Use `init` when you want an editable contract. By default it writes
`config.schema.yaml` and `config_gen.go` under the package directory, which is
`appconfig` unless you pass `--package`. Pass explicit paths when needed. There
is no overwrite flag; use new destinations when trying another inference.

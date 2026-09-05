# Schema parser

The build-time parser is available in `go-config/schema`. The module path is
temporary until a publication location is chosen.

```go
data, err := os.ReadFile("config.schema.yaml")
if err != nil {
    return err
}
parsed, err := schema.Parse("config.schema.yaml", data)
if err != nil {
    return err
}
for _, field := range parsed.Fields {
    fmt.Println(field.Path, field.Type)
}
```

The filename is a diagnostic label. `Parse` reads the supplied bytes and does
not open a file. Fields and nested fields retain declaration order. List item
paths use `[]` and map value paths use `{}` to identify schema definitions.
These paths do not yet define runtime collection addressing.

`Field.Has("required")` distinguishes an omitted property from `required: false`.
Defaults and constraints remain YAML nodes, preserving their type and source
position. An explicit `default: null` is preserved for rejection by semantic
validation; it does not become an absent default.

Inspect errors with `errors.As(err, &diagnostic)` where `diagnostic` has type
`*schema.Diagnostic`. Structural errors carry source positions. YAML decoder
failures currently use line 1, column 1 and a sanitized message so parser errors
cannot echo secret scalar values. Errors stop at the first structural problem.

Parsing does not yet check supported versions, valid Go identifiers, type
compatibility, defaults, constraints, or naming collisions. Those checks belong
to phase 2. There is no code generator or runtime loader yet.

Run the parser tests and fuzz smoke test:

```sh
go test ./schema
go test ./schema -run '^$' -fuzz '^FuzzParse$' -fuzztime=5s -parallel=2
```

# Error and Diagnostics Specification

## 1. Goals

Configuration failures must be:

- useful to humans;
- inspectable by Go programs;
- stable enough for callers to classify;
- safe for secret fields;
- capable of aggregating independent problems.

## 2. Runtime error model

Recommended public shape:

```go
type Error struct {
    Issues []Issue
}

type Issue struct {
    Path     string
    Source   string
    Kind     IssueKind
    Message  string
    Location *Location
}

type Location struct {
    File   string
    Line   int
    Column int
}
```

Do not store raw secret values in public `Issue` fields.

## 3. Issue kinds

Expose stable categories:

```go
type IssueKind string

const (
    IssueSource       IssueKind = "source"
    IssueSyntax       IssueKind = "syntax"
    IssueDuplicate    IssueKind = "duplicate"
    IssueUnknownField IssueKind = "unknown_field"
    IssueType         IssueKind = "type"
    IssueRequired     IssueKind = "required"
    IssueConstraint   IssueKind = "constraint"
    IssueCanceled     IssueKind = "canceled"
)
```

Exact string values can differ before v1.0, but the categories themselves should remain.

## 4. `errors.As`

Callers must be able to inspect configuration errors:

```go
var cfgErr *config.Error
if errors.As(err, &cfgErr) {
    for _, issue := range cfgErr.Issues {
        // classify issue.Kind, issue.Path, issue.Source
    }
}
```

## 5. Human formatting

Single issue may render compactly:

```text
configuration error: server.port: value 70000 exceeds maximum 65535
```

Multiple issues should render as a readable list:

```text
configuration error:
  database.dsn: required value is missing
  server.port: value 70000 exceeds maximum 65535
  server.timeout: invalid duration "thirty seconds"
```

## 6. Provenance

When source is useful, include it:

```text
configuration error:
  server.port:
    source: env MYAPP_SERVER_PORT
    expected: int
    value: "abc"
```

File syntax error:

```text
config.yaml:17:8: invalid YAML syntax
```

Unknown field:

```text
config.yaml:8:3: unknown configuration field "server.prot"
```

## 7. Secret redaction

For a secret field:

```text
configuration error:
  database.password:
    source: env DATABASE_PASSWORD
    expected: value matching configured pattern
    value: [REDACTED]
```

Never include the raw secret in `Message`, `Error()`, debug formatting, or nested wrapped errors created by this library.

If an underlying parser error string could contain a secret value, sanitize or replace it before exposure.

## 8. Aggregation

Source-level failures that prevent meaningful continuation may stop that source/load immediately, e.g. unreadable required file or malformed YAML structure.

Final conversion/required/constraint checks should aggregate independent leaf issues where practical.

Ordering should be deterministic, preferably schema order, with source-level errors ordered by source order.

## 9. Wrapping underlying errors

Preserve useful underlying errors for `errors.Is`/`errors.As` where safe, especially `context.Canceled`, `context.DeadlineExceeded`, and filesystem errors.

Do not expose unsafe raw values when wrapping conversion failures on secret fields.

## 10. Schema/generator diagnostics

Schema errors are build-time diagnostics and should include field path and line/column when possible.

Examples:

```text
config.schema.yaml:18:5: field "server.port": unknown type "integer32"
config.schema.yaml:22:5: field "server.port": min cannot exceed max
config.schema.yaml:37:5: environment variable DATABASE_URL is already mapped by "database.read_url"
config.schema.yaml:44:5: default "abc" is not a valid duration
```

## 11. No panic traces for ordinary user mistakes

`configgen` should exit cleanly with formatted diagnostics. Runtime `Load` returns errors. Only generated `MustLoad` intentionally panics on a returned load error.

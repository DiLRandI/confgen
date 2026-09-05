# Test Plan

## 1. Test strategy

Use four layers:

1. unit tests for schema, merge, conversion, validation, and error formatting;
2. integration tests for built-in sources;
3. codegen compilation tests;
4. end-to-end generated-package tests.

Also run race tests and selected fuzz tests.

## 2. Required acceptance cases

The following are release-blocking for v1:

1. YAML file loads successfully.
2. JSON file loads successfully.
3. Environment-only configuration loads.
4. Environment overrides file when later.
5. File overrides environment when later.
6. Defaults work.
7. Explicit `false` satisfies required bool.
8. Explicit `0` satisfies required integer.
9. Explicit empty string satisfies required string.
10. Missing required field fails.
11. Invalid duration fails.
12. Valid composite duration succeeds.
13. Numeric minimum works.
14. Numeric maximum works.
15. Enum validation works.
16. Pattern validation works.
17. Unknown YAML field fails by default.
18. Unknown JSON field fails by default.
19. Unknown file fields can be ignored when schema says so.
20. Empty env string counts as present.
21. Invalid env integer reports variable name.
22. Secret values are redacted.
23. Nested object override happens by leaf.
24. Lists replace rather than merge.
25. Maps replace rather than deep-merge.
26. Optional missing file succeeds.
27. Required missing file fails.
28. Permission/read errors are not swallowed.
29. Duplicate YAML key fails.
30. Duplicate JSON key fails.
31. Invalid schema default fails generation.
32. Duplicate generated Go names fail generation.
33. Duplicate environment names fail generation.
34. Generated code is gofmt-formatted.
35. Generation is deterministic.
36. Generated code compiles.
37. Reader YAML source works.
38. Reader JSON source works.
39. Lower-priority type-invalid raw value can be replaced by valid higher-priority value.
40. Effective type-invalid value fails.
41. Effective YAML/JSON null fails.
42. Context cancellation prevents successful result.
43. Multiple final validation issues aggregate deterministically.
44. Map/list environment variables use JSON syntax.
45. Disabled env mapping is ignored.
46. Explicit env-name override works.

## 3. Schema unit tests

Cover:

- version missing/unsupported;
- invalid package/name identifiers;
- invalid field-key regex;
- unknown field/root properties;
- every supported type;
- invalid type names;
- object without fields;
- list without items;
- map without values;
- map-of-object rejection in v1;
- property/type compatibility;
- `required` rejection on objects;
- invalid regex;
- duplicate enum values;
- invalid enum value types;
- min/max ordering;
- min_length/max_length ordering;
- invalid defaults;
- Go-name initialisms;
- `go_name` overrides;
- Go-name collisions;
- env-name generation;
- env-name collisions;
- env disabled fields;
- duplicate YAML keys in schema.

## 4. Merge unit tests

Use synthetic `Document` values to test:

- no sources;
- one source;
- multiple sources;
- absent field does not override;
- explicit zero overrides;
- explicit false overrides;
- explicit empty string overrides;
- nested leaf override;
- list atomic replacement;
- map atomic replacement;
- provenance follows winner.

## 5. Conversion tests

Table-test every scalar kind for file/raw and env text inputs.

Include integer overflow/underflow, negative unsigned values, NaN/infinity policy for floats, and invalid duration strings.

Recommended float policy: allow values accepted by `strconv.ParseFloat` only if finite; reject NaN and infinities unless deliberately specified otherwise. Lock this decision before v1 release.

## 6. File parser tests

Fixtures for:

- normal YAML;
- normal JSON;
- malformed YAML;
- malformed JSON;
- duplicate YAML mapping keys;
- duplicate JSON object keys;
- unknown nested fields;
- unknown-field ignore mode;
- multiple YAML documents rejected;
- null effective value;
- empty configuration object.

## 7. Environment tests

Use `t.Setenv` where available. Cover:

- missing vs empty;
- bool variants accepted by `strconv.ParseBool`;
- integers;
- floats;
- duration;
- list JSON;
- map JSON;
- list-of-object JSON;
- explicit env mapping;
- env disabled;
- unrelated environment ignored.

## 8. Error tests

Assert:

- `errors.As` works;
- issue kind/path/source are populated;
- deterministic multi-error order;
- file locations when available;
- secret text is absent from `err.Error()`;
- secret text is absent from `%+v` or any library-owned formatting of error types.

Add a regression test using a unique sentinel secret string and assert it does not occur anywhere in rendered diagnostics.

## 9. Codegen golden tests

Use fixture schemas and golden generated `.go`, YAML-example, and env-example outputs.

Golden tests should be readable and intentionally updated.

## 10. Compilation integration test

For representative schemas:

1. generate Go output into a temporary module/package;
2. write minimal `go.mod` using a replace directive to local runtime source;
3. run `go test ./...`;
4. fail if generated code does not compile.

## 11. End-to-end test

Schema:

```yaml
version: 1
package: testconfig
env_prefix: APP
fields:
  host:
    type: string
    default: localhost
  port:
    type: int
    default: 8080
    min: 1
    max: 65535
  timeout:
    type: duration
    default: 30s
  database_url:
    type: string
    required: true
    secret: true
```

File:

```yaml
port: 9000
```

Env:

```text
APP_DATABASE_URL=postgres://secret
APP_PORT=10000
```

Expected result:

```go
Config{
    Host:        "localhost",
    Port:        10000,
    Timeout:     30 * time.Second,
    DatabaseURL: "postgres://secret",
}
```

## 12. Race tests

CI should run:

```bash
go test -race ./...
```

Add a test that concurrently loads the same generated config with immutable file/reader inputs.

## 13. Fuzz tests

Useful fuzz targets:

- schema YAML parser must never panic;
- YAML config normalization must never panic;
- JSON duplicate-key parser must never panic;
- environment scalar conversion must never panic;
- canonical path traversal must never panic.

Fuzz failures should become fixed regression fixtures.

## 14. CI gates

Minimum CI:

```bash
gofmt check
go vet ./...
go test ./...
go test -race ./...
```

Optionally add `staticcheck` after the core implementation stabilizes.

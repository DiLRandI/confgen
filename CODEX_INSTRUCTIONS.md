# Codex Implementation Instructions

## Mission

Implement the Go configuration library described by the documents in this package.

Treat these files as the implementation contract. When documents overlap, the more specific document wins. If a genuine contradiction remains, preserve the non-negotiable product semantics below and choose the simplest implementation consistent with them.

## Read first

Read in this order:

1. `README.md`
2. `ARCHITECTURE.md`
3. `SCHEMA_SPEC.md`
4. `RUNTIME_SPEC.md`
5. `CODEGEN_SPEC.md`
6. `ERROR_SPEC.md`
7. `TEST_PLAN.md`
8. `IMPLEMENTATION_PLAN.md`

## Non-negotiable product semantics

Do not change these:

1. Schema is authored once in YAML and is authoritative.
2. Go config structs are generated from the schema.
3. Runtime never parses the schema YAML.
4. Source order is left-to-right; later sources win.
5. Defaults are lower priority than every runtime source.
6. Presence is separate from zero value.
7. `required` means present after merge, not non-zero.
8. Empty env variables count as present.
9. Objects merge by leaf path.
10. Lists replace atomically.
11. Maps replace atomically.
12. Unknown file fields error by default.
13. Duplicate YAML/JSON keys are errors.
14. Effective `null` is unsupported in v1.
15. Duration uses `time.ParseDuration` semantics.
16. File values are strongly typed; env values are textual.
17. Final type conversion occurs after precedence merge.
18. `Load` returns an error; `MustLoad` panics on that error.
19. Secret values never appear in library-generated diagnostics.
20. No global mutable configuration registry.
21. No `unsafe` required for v1.
22. Generated output is deterministic.

## Scope discipline

Do not add these to v1:

- flags source;
- TOML;
- `.env` parsing;
- Vault/Consul/etcd/SSM/Kubernetes providers;
- hot reload;
- dynamic `Get` APIs;
- templating/interpolation;
- pointer/nullability system;
- configuration UI.

If extensibility is needed, expose only the minimal interfaces necessary for future third-party `Source` implementations.

## Coding approach

Follow `IMPLEMENTATION_PLAN.md` phases. Do not start with code generation before schema validation and runtime contracts are tested.

Prefer small packages and ordinary Go. Avoid framework-style abstractions.

Use table-driven tests heavily.

For every fixed bug involving parsing, precedence, presence, duplicate keys, or redaction, add a regression test.

## Dependency guidance

Prefer:

- Go standard library;
- `gopkg.in/yaml.v3` for YAML parsing;
- no provider-specific dependencies.

Do not add a large validation library; the v1 constraint system is small and should be implemented directly.

## Error handling

Ordinary configuration/schema mistakes must return formatted errors, not panic.

Generated `MustLoad` is the intended exception.

Never include a secret raw value in errors, including wrapped underlying errors.

## Code quality gates

Before considering a phase complete, run:

```bash
gofmt -w .
go vet ./...
go test ./...
go test -race ./...
```

Keep generated fixtures formatted and deterministic.

## Completion definition

The implementation is complete only when the end-to-end flow works:

```text
config.schema.yaml
  -> configgen
  -> generated Config + descriptor
  -> YAML/JSON/ENV sources
  -> ordered merge
  -> validation
  -> typed Config
```

and all required cases in `TEST_PLAN.md` pass.

# Go Configuration Library — Implementation Handoff

This package is the implementation contract for a small schema-first Go configuration library.

## Product promise

Define configuration once in `config.schema.yaml`, generate strongly typed Go code, and load values from ordered YAML, JSON, and environment sources with deterministic precedence.

The intended application experience is deliberately small:

```go
cfg := appconfig.MustLoad(
    config.OptionalFile("config.yaml"),
    config.Env(),
)
```

Later sources win. Schema defaults are conceptually the lowest-priority source.

## Read these documents in order

1. `ARCHITECTURE.md` — system boundaries, data flow, packages, and invariants.
2. `SCHEMA_SPEC.md` — authoritative schema grammar and validation rules.
3. `RUNTIME_SPEC.md` — sources, merge semantics, conversion, validation, and public runtime API.
4. `CODEGEN_SPEC.md` — `configgen`, generated Go code, deterministic output, and examples generation.
5. `ERROR_SPEC.md` — structured error model, issue categories, formatting, and secret redaction.
6. `TEST_PLAN.md` — unit, integration, acceptance, race, and fuzz coverage.
7. `IMPLEMENTATION_PLAN.md` — phased implementation order and completion gates.
8. `CODEX_INSTRUCTIONS.md` — concrete coding-agent instructions and non-negotiable decisions.
9. `MASTER_SPEC.md` — compact end-to-end product contract for reference during implementation.

The `examples/` directory contains a representative schema, YAML config, JSON config, and environment example.

## In scope for v1

- YAML schema as the single source of truth.
- Generated Go configuration types and descriptors.
- YAML configuration files.
- JSON configuration files.
- Environment variables.
- Explicit source ordering where later sources override earlier sources.
- Defaults.
- Presence-based `required` validation.
- Scalars, nested objects, lists, and string-keyed maps.
- Go-standard durations using `time.ParseDuration`.
- Secret metadata and redaction in library-generated diagnostics.
- Unknown-field detection.
- Duplicate-key detection.
- Structured, inspectable errors.
- Generated `Load`, `LoadContext`, and `MustLoad` helpers.
- Optional example YAML and environment generation.

## Explicitly out of scope for v1

- CLI flags as a configuration source.
- TOML.
- `.env` file parsing.
- Consul, etcd, Vault, AWS SSM, Kubernetes providers.
- Hot reload or file watching.
- Runtime dynamic getters such as `GetString`.
- Configuration interpolation or templating.
- Nullable/pointer field generation.
- Remote configuration.
- Dependency injection.
- Configuration UI.

The `Source` interface must remain extensible so third-party sources can be added later without expanding the core package.

## Non-negotiable invariants

- The schema file is the configuration contract.
- Runtime code never reparses the schema file.
- Later runtime sources win.
- Defaults are lower priority than every runtime source.
- Required validation checks presence, never Go zero values.
- Empty environment variables count as present.
- Objects merge by leaf path.
- Lists replace atomically.
- Maps replace atomically.
- Unknown file fields are errors by default.
- YAML/JSON `null` is unsupported in v1.
- Secret values are never emitted by library-generated diagnostics.
- Generated output is deterministic and formatted.
- No global mutable configuration registry.

## Suggested repository layout

```text
/
├── config/                 # Public runtime package
├── schema/                 # Schema parser and validation
├── generator/              # Go and example-file generation
├── cmd/configgen/          # CLI
├── internal/testutil/      # Shared test helpers
├── examples/               # End-to-end example inputs
└── docs/                   # Optional user-facing docs later
```

## Positioning

Do not position this project as "a smaller Viper." The intended differentiation is:

> A schema-first configuration generator for Go. Define your configuration once, generate strongly typed Go code, and safely compose YAML, JSON, and environment sources with deterministic precedence.

Short tagline:

> Typed Go configuration without writing the same configuration twice.

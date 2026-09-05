# Implementation Plan

## 1. Objective

Implement the v1 library in small, independently testable phases. Do not begin with a giant end-to-end generator. First establish schema IR and runtime contracts.

## Phase 0 — Repository foundation

Create:

```text
config/
schema/
generator/
cmd/configgen/
internal/testutil/
examples/
```

Set up `go.mod`, baseline CI, formatting checks, and package docs.

**Gate:** `go test ./...` succeeds with empty/scaffold packages.

## Phase 1 — Schema AST and parser

Implement:

- YAML node parsing with `yaml.v3`;
- single-document enforcement;
- duplicate-key detection;
- root model;
- recursive field model;
- unknown-property rejection;
- source locations.

Do not yet generate Go.

**Gate:** schema parser tests cover syntax, duplicates, and unknown properties.

## Phase 2 — Schema semantic validation

Implement:

- supported type checks;
- field-name regex;
- property compatibility;
- objects/lists/maps rules;
- defaults;
- constraints;
- regex compilation validation;
- Go-name generation;
- initialism handling;
- canonical paths;
- env-name generation;
- collision detection.

Produce a normalized internal representation suitable for codegen.

**Gate:** all schema unit tests pass.

## Phase 3 — Runtime descriptor API

Define stable-enough descriptor structures and kind enums.

Implement tests constructing descriptors manually before codegen exists.

**Gate:** descriptors can describe all v1 schema shapes without runtime parsing of schema files.

## Phase 4 — Runtime effective-state and merge

Implement:

- presence-aware raw values;
- source provenance;
- defaults seeding;
- ordered merge;
- leaf-path behavior;
- atomic list/map replacement.

Use synthetic test sources.

**Gate:** merge unit tests pass, including zero/false/empty behavior.

## Phase 5 — Conversion and final validation

Implement conversion for all v1 types and constraints.

Implement:

- required;
- min/max;
- min_length/max_length;
- enum;
- pattern;
- null rejection;
- structured error aggregation;
- secret redaction.

**Gate:** conversion/validation/error tests pass without file/env sources.

## Phase 6 — File/reader sources

Implement:

- YAML source parser;
- JSON source parser;
- duplicate-key detection;
- unknown-field detection;
- canonical path normalization;
- line/column provenance where possible;
- `File`;
- `OptionalFile`;
- `Reader`.

**Gate:** YAML/JSON source tests pass.

## Phase 7 — Environment source

Implement:

- `os.LookupEnv` semantics;
- generated env-name consumption;
- disabled env mapping;
- explicit env mapping;
- scalar textual values;
- JSON list/map values.

Avoid eager conversion if it would violate effective-value-after-merge semantics. Preserve a source encoding marker so final conversion knows env values are textual.

**Gate:** environment tests pass.

## Phase 8 — Generic loader and assignment

Implement:

```go
config.Load[T](ctx, descriptor, sources...)
```

Populate generated types using descriptor-guided reflection without `unsafe`.

Guarantee `nil, err` on failure.

**Gate:** manually authored descriptor + manually authored struct end-to-end runtime tests pass.

## Phase 9 — Go generator

Generate:

- root `Config` type;
- nested named types;
- struct tags;
- descriptor;
- `Load`;
- `LoadContext`;
- `MustLoad`.

Format with `go/format`. Implement deterministic output and atomic writes.

**Gate:** golden and compilation tests pass.

## Phase 10 — CLI

Implement `cmd/configgen` flags and clean diagnostics.

**Gate:** CLI integration tests cover success and schema failure exit codes.

## Phase 11 — Example generation

Implement optional:

- YAML example;
- env example.

Preserve schema order and safe secret placeholders.

**Gate:** golden example tests pass.

## Phase 12 — Full acceptance suite

Run every case in `TEST_PLAN.md`, race tests, and fuzz smoke tests.

**Gate:** v1 MVP acceptance criteria all pass.

## 2. Recommended implementation choices

- `gopkg.in/yaml.v3` nodes for schema and YAML duplicate detection.
- Standard `encoding/json.Decoder` token traversal for duplicate JSON-key detection.
- A normalized schema IR separate from raw YAML structs.
- A runtime `RawValue` carrying an encoding/source kind so file values and env textual values convert differently.
- Deterministic slices rather than maps for schema-ordered descriptor rendering where practical.
- `go/format` for Go output.

## 3. Decisions to lock before public v1

These are smaller design details that Codex may choose initially but should be documented before release:

- exact runtime module/package import path;
- whether non-finite float values are rejected (recommended: reject);
- whether `Reader` snapshots bytes at construction (recommended: yes);
- exact public descriptor visibility;
- exact issue-kind string values;
- whether generated nested type naming uses path concatenation or parent-name suffixing.

None may change the core semantics from the other documents.

## 4. Do not optimize early

Configuration loads at startup. Prioritize correctness, diagnostics, maintainability, and predictable behavior over micro-optimizations.

Reflection is acceptable in v1. Do not introduce `unsafe` to avoid it.

## 5. Pull-request sequencing

Suggested PRs:

1. repository + schema parser;
2. schema semantics + normalized IR;
3. runtime descriptor/merge;
4. runtime conversion/validation/errors;
5. YAML/JSON sources;
6. env source;
7. generic loader/assignment;
8. code generator;
9. CLI + example generation;
10. acceptance hardening/docs.

Each PR should leave `main` compiling and tested.

# Implementation progress

Completion is measured against the 13 phases in
[IMPLEMENTATION_PLAN.md](../IMPLEMENTATION_PLAN.md). A checked phase means its
gate and the repository formatting, vet, test, and race checks passed locally.
This is phase completion, not an estimate of effort or release readiness.

- [x] Phase 0: repository foundation
- [x] Phase 1: schema AST and parser
- [ ] Phase 2: schema semantic validation
- [ ] Phase 3: runtime descriptor API
- [ ] Phase 4: effective state and merge
- [ ] Phase 5: conversion and final validation
- [ ] Phase 6: file and reader sources
- [ ] Phase 7: environment source
- [ ] Phase 8: generic loader and assignment
- [ ] Phase 9: Go generator
- [ ] Phase 10: CLI
- [ ] Phase 11: example generation
- [ ] Phase 12: full acceptance suite

Current completion: 2/13 phases. No runtime loader or generator is available yet.
Create CLI and test-helper packages when they contain working code.

## Implemented parser

`schema.Parse(filename, data)` returns an ordered AST or a `*schema.Diagnostic`.
It enforces one YAML document, string mapping keys, required root properties,
known root and recursive field properties, and typed metadata. It rejects
duplicate keys throughout the document, including defaults, and preserves
property presence and YAML value nodes for semantic validation.

Use [the parser guide](schema-parser.md) for the API and its current limits.
The next phase is semantic validation of types, constraints, defaults, names,
and environment mappings. Parsed schemas must not yet be used for generation.

## Verification

Run from the repository root:

```sh
gofmt -w .
go vet ./...
go test ./...
go test -race ./...
```

These checks passed locally on Go 1.27.1. A five-second schema fuzz smoke run
also passed with 14,072 executions. CI is configured but has not run remotely.

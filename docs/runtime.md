# Advanced loading

Generated packages expose `Load`, `LoadContext`, and `MustLoad`. `LoadContext`
passes cancellation to sources; `MustLoad` panics on error. No failed load returns
partial configuration. Blocking filesystem reads cannot be interrupted.

`File` and `Env` read at load time. `Reader` snapshots input when constructed and
can be reused. Generated descriptors are immutable; returned collections are
independent between loads. Shared custom sources must support concurrent calls.

Only winning values are converted. A later valid value can replace an earlier
invalid type or null. File syntax, duplicate-key, and unknown-field errors cannot
be hidden this way. YAML aliases and merge keys are unsupported.

Use `errors.As` with `*config.Error` to inspect ordered issues. Filesystem and
context errors support `errors.Is`. Custom source authors should read the GoDoc
for `Source`, `Document`, and `RawValue` and keep their diagnostics secret-safe.

## Input byte budgets

File and reader sources accept up to 32 MiB per document by default. Readers
consume at most the budget plus one byte, discard partial data on failure, and
report failures when loaded. Files are bounded while reading, including files
that grow. An oversized source returns `*config.Error` with `IssueSource` and
an underlying `*input.SizeError`; diagnostics contain the budget, not contents.

Use `config.ReaderWithLimit`, `config.FileWithLimit`, or
`config.OptionalFileWithLimit` with an `input.Limit` for a different budget.
`config.WithEnvInputLimit` sets the budget for each environment value. Zero uses
`input.DefaultLimit`; positive limits can be smaller or larger; negative limits
are rejected. Limits are per source, not a total memory quota. Parsed structures
can require more memory than their encoded bytes. Custom `Source` implementations
own their input budgets, including text they return for collection parsing.

Schema parsing and compilation use the same default. Use `schema.ParseWithLimit`
or `schema.CompileWithLimit` to override it. Inference uses `infer.Options.InputLimit`;
overrides have `infer.ParseOverridesWithLimit`. Byte-slice APIs check before
parsing, but callers still own the allocation of those slices. `input.Limit.Read`
and `ReadFile` let callers bound that allocation too. The CLI bounds schema,
inference, and override files to the default 32 MiB. Generated intermediate
schemas are validated without applying the source byte budget a second time.

The default leaves room for large configuration files while preventing endless
or unexpectedly huge reads. It is an input budget, not a parser execution timeout;
reader snapshots still cannot interrupt an underlying blocking read.

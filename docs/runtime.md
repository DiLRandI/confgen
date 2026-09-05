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

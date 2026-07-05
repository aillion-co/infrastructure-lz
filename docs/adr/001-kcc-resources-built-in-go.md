# ADR-001: KCC resources are built in Go code, not template files

## Status

Accepted

## Context

The IAC generator produces Google Config Connector (KCC) YAML manifests for
each activation feature. Two approaches were considered:

1. **External template files** — a `templates/` directory of `.yaml.tmpl`
   files loaded at runtime.
2. **Templates embedded as Go constants** — each feature builder
   (`internal/generator/kcc/*.go`) declares its manifests as `text/template`
   constants next to the validation and assembly logic for that feature.

## Decision

KCC manifests are declared as Go string constants inside their feature
builder, rendered through the shared `renderTemplate` helper and the
`templateFuncs` function map (`internal/generator/kcc/feature_builder.go`).

Reasons:

- **Single compilation unit.** The binary is self-contained; there is no
  runtime file loading to fail in the distroless container image.
- **Locality.** A feature's config struct, validation, template, and golden
  test live together, so a change to one is reviewed with the others.
- **Type-checked data flow.** Template fields reference the feature's config
  struct; renaming a field breaks the golden tests immediately.
- **Testability.** Every builder's output is covered by golden-file tests in
  `internal/generator/kcc/testdata/golden/`, compared byte-for-byte.

## Consequences

- Editing a manifest requires a recompile (acceptable: output changes must
  run the golden tests anyway).
- Templates cannot be customized per deployment. Per-customer variation is
  expressed through feature config fields, not template forks.
- The `internal/generator/templates/` directory from the original project
  skeleton was removed; new manifests belong in their feature builder file.

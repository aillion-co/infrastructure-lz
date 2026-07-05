# ADR-002: Activation output is one umbrella Helm chart with a sub-chart per feature

## Status

Accepted

## Context

An activation enables several features (bootstrap-org, bigquery-analytics,
secure-inferencing, …), each producing multiple KCC manifests. The download
needs a structure that customers can apply, inspect, and selectively disable.

Options considered: a flat directory of YAML files, one Helm chart per
feature delivered separately, or a single umbrella chart with per-feature
sub-charts.

## Decision

`internal/generator/helm.ActivationBuilder` emits one umbrella chart named
`<customer>-activation` containing:

```
<customer>-activation/
├── Chart.yaml          # dependencies: one entry per enabled feature
├── values.yaml         # <feature>.enabled toggle per feature
└── charts/
    └── <feature-id>/
        ├── Chart.yaml
        ├── values.yaml
        └── templates/  # _helpers.tpl + the feature's KCC manifests
```

- Feature IDs are emitted in sorted order so identical requests produce
  byte-identical charts (reproducible output, diffable across runs).
- Helm value keys derive from feature IDs with hyphens mapped to
  underscores (`sanitizeHelmKey`), since Helm values must be valid
  identifiers.
- Each sub-chart is gated by a `<feature>.enabled` condition in the umbrella
  `Chart.yaml`, so customers can switch features off at install time
  without editing manifests.

## Consequences

- One `helm install` applies an entire activation; `--set
  <feature>.enabled=false` excludes a feature.
- Feature dependencies (e.g. everything depends on bootstrap-org) are
  validated server-side at request time (`validateActivationRequest`), not
  encoded in Helm, because Helm has no cross-sub-chart dependency gating.
- The zip layout is part of the API contract and is asserted by the
  integration suite (`assertValidUmbrellaChart`).

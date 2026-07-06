# ADR-003: Governance guardrails as OPA Gatekeeper policies shipped with every activation

## Status

Accepted

## Context

Generated landing zones must keep the end user compliant after deployment,
not just at generation time. The guardrails therefore have to live inside
the deployed system and enforce themselves, and customers operate under
different regulatory regimes — CIS Google Cloud Foundations, GDPR, PCI DSS,
ISO/IEC 27001, NIS2, and the EU Cyber Resilience Act — that they must be
able to select per activation.

## Decision

A `governance-guardrails` activation feature generates an OPA-based policy
sub-chart (`internal/generator/kcc/governance.go`):

- **Enforcement engine.** The sub-chart enables the fleet **GKE Policy
  Controller** (Google-managed OPA Gatekeeper) through a KCC
  `GKEHubFeature`, plus the `gkehub` and `anthospolicycontroller` service
  APIs. Because guardrails run as admission control on the config cluster,
  non-compliant KCC resources are rejected *before* Config Connector
  actuates them in GCP — the landing zone polices its own changes from the
  moment it is applied.
- **Policy catalog.** Guardrails are Gatekeeper `ConstraintTemplate` +
  `Constraint` pairs written in Rego, held in a fixed-order catalog where
  each policy is tagged with the regimes it serves. Selecting regimes
  yields the union of matching policies; each emitted constraint carries a
  `compliance.aillion.co/regimes` annotation naming the regimes that pulled
  it in.
- **Regime selection.** The wizard exposes regime checkboxes (and the API
  accepts `config.regimes`), an enforcement mode (`deny` or `dryrun` for
  audit-only rollout), an approved-region list for the data-residency
  guardrail (EU defaults), and the label set required on resources.
- **Compliance evidence.** A generated `regime-manifest.yaml` ConfigMap
  records the selected regimes and the exact policies each resolved to, so
  auditors can trace regime → control → constraint inside the cluster.

## Consequences

- Guardrails apply to infrastructure-as-data (KCC resources) at admission;
  they do not retroactively scan resources created outside the config
  cluster. Periodic audit of existing objects comes free with Gatekeeper's
  audit controller once Policy Controller is enabled.
- New regimes or policies are added by extending the catalog in
  `governance.go` and tagging regimes; golden tests pin the rendered
  output.
- `dryrun` mode lets customers observe violations before enforcing, which
  is the recommended adoption path for brownfield estates.

# ADR-004: Golden architecture patterns ship in the Backstage portal, aligned with governance regimes

## Status

Accepted

## Context

A landing zone on its own tells teams what they *cannot* do (via the
ADR-003 governance guardrails) but not what they *should* do. Teams need
paved-road architectures for the most common GCP systems, and those
architectures must be compliant with the customer's selected regimes by
construction — otherwise the first thing a team builds fights the
guardrails.

## Decision

The `dynamic-developer-portal` feature (Backstage) ships a golden-pattern
catalog out of the box (`internal/generator/kcc/golden_patterns.go`):

- **Delivery.** Patterns are Backstage scaffolder `Template` entities
  rendered into a `backstage-golden-patterns` ConfigMap, with an
  `app-config` fragment (`backstage-app-config-patterns`) registering each
  file as a catalog location. The portal feature deploys a Backstage
  application `Deployment`/`Service` that mounts the pattern ConfigMap at
  `/golden-patterns` and loads the app-config fragment via `--config`, so
  the patterns appear in the portal as soon as the landing zone deploys.
  The portal itself is recommended by default, so a standard activation
  includes the dashboard.
- **Out-of-the-box patterns.** Three-tier web application, serverless
  event-driven processing, EU data platform, private GKE microservices,
  secure ML inference, and regulated payments processing — the most common
  GCP system shapes.
- **Regime alignment.** Every pattern declares the regimes it is designed
  for and the exact guardrail policies (ADR-003 catalog IDs) it stays
  within, surfaced as tags and `compliance.aillion.co/*` annotations on
  the Template entity. Unit tests fail the build if a pattern references
  a regime or guardrail that does not exist in the governance catalog, and
  if any selectable regime has no covering pattern.

## Consequences

- Pattern and guardrail libraries cannot drift: the cross-reference is
  test-enforced in one binary.
- Scaffolder steps reference `./skeletons/<pattern-id>` in the portal's
  configured Git repository; populating those skeleton repos is customer
  onboarding work, not generator output.
- New patterns are added to the catalog in `golden_patterns.go` with
  regime/guardrail tags; golden-file tests pin the rendered entities.

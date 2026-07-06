package kcc

import (
	"fmt"
	"sort"
	"strings"

	"github.com/aillion-co/infrastructure-lz/internal/models"
)

// governanceRegimes maps regime identifiers to display names. These are the
// compliance regimes a user can select; each selected regime pulls in the
// guardrail policies tagged with it.
var governanceRegimes = map[string]string{
	"cis":      "CIS Google Cloud Foundations Benchmark",
	"gdpr":     "GDPR",
	"pci-dss":  "PCI DSS",
	"iso27001": "ISO/IEC 27001",
	"nis2":     "NIS2",
	"eucra":    "EU Cyber Resilience Act",
}

// governancePolicy is one OPA Gatekeeper guardrail: a ConstraintTemplate
// (Rego) plus a Constraint binding it to KCC resource kinds. Policies are
// admission-time checks on Config Connector resources, so non-compliant
// infrastructure is rejected before it is ever actuated in GCP.
type governancePolicy struct {
	ID          string // constraint name and policy-<ID>.yaml file name
	Kind        string // ConstraintTemplate CRD kind (CamelCase)
	Description string
	Regimes     []string // regime IDs that include this policy
	TargetKinds []string // KCC kinds the constraint matches
	Rego        string
	// Parameters renders extra YAML under the constraint's spec.parameters.
	// Built per-config where the policy is parameterized (nil otherwise).
	Parameters func(cfg *models.GovernanceConfig) map[string][]string
}

// governanceCatalog is the full guardrail catalog in a fixed order so the
// generated chart bytes are deterministic. Selecting regimes yields the
// union of policies tagged with any selected regime.
var governanceCatalog = []governancePolicy{
	{
		ID:          "no-public-iam",
		Kind:        "GcpNoPublicIam",
		Description: "Deny IAM bindings that grant access to allUsers or allAuthenticatedUsers",
		Regimes:     []string{"cis", "gdpr", "iso27001", "nis2", "pci-dss"},
		TargetKinds: []string{"IAMPolicy", "IAMPolicyMember"},
		Rego: `package gcpnopubliciam

violation[{"msg": msg}] {
  member := public_members[_]
  msg := sprintf("public IAM member %v is not permitted", [member])
}

public_members[member] {
  member := input.review.object.spec.member
  is_public(member)
}

public_members[member] {
  member := input.review.object.spec.bindings[_].members[_]
  is_public(member)
}

is_public(member) { member == "allUsers" }
is_public(member) { member == "allAuthenticatedUsers" }`,
	},
	{
		ID:          "storage-uniform-access",
		Kind:        "GcpStorageUniformAccess",
		Description: "Require uniform bucket-level access on Cloud Storage buckets",
		Regimes:     []string{"cis", "iso27001"},
		TargetKinds: []string{"StorageBucket"},
		Rego: `package gcpstorageuniformaccess

violation[{"msg": msg}] {
  input.review.object.kind == "StorageBucket"
  not input.review.object.spec.uniformBucketLevelAccess == true
  msg := "StorageBucket must set spec.uniformBucketLevelAccess: true"
}`,
	},
	{
		ID:          "require-cmek",
		Kind:        "GcpRequireCmek",
		Description: "Require customer-managed encryption keys on storage, BigQuery, and Cloud SQL",
		Regimes:     []string{"gdpr", "iso27001", "nis2", "pci-dss"},
		TargetKinds: []string{"StorageBucket", "BigQueryDataset", "SQLInstance"},
		Rego: `package gcprequirecmek

violation[{"msg": msg}] {
  obj := input.review.object
  obj.kind == "StorageBucket"
  not obj.spec.encryption.kmsKeyRef
  msg := "StorageBucket must reference a customer-managed KMS key (spec.encryption.kmsKeyRef)"
}

violation[{"msg": msg}] {
  obj := input.review.object
  obj.kind == "BigQueryDataset"
  not obj.spec.defaultEncryptionConfiguration.kmsKeyRef
  msg := "BigQueryDataset must reference a customer-managed KMS key (spec.defaultEncryptionConfiguration.kmsKeyRef)"
}

violation[{"msg": msg}] {
  obj := input.review.object
  obj.kind == "SQLInstance"
  not obj.spec.encryptionKMSCryptoKeyRef
  msg := "SQLInstance must reference a customer-managed KMS key (spec.encryptionKMSCryptoKeyRef)"
}`,
	},
	{
		ID:          "allowed-regions",
		Kind:        "GcpAllowedRegions",
		Description: "Restrict resource locations to an approved region list (data residency)",
		Regimes:     []string{"gdpr", "nis2"},
		TargetKinds: []string{"*"},
		Rego: `package gcpallowedregions

violation[{"msg": msg}] {
  location := object_location
  not location_allowed(location)
  msg := sprintf("location %v is not in the approved region list %v", [location, input.parameters.regions])
}

object_location = location {
  location := input.review.object.spec.location
} else = location {
  location := input.review.object.spec.region
}

location_allowed(location) {
  lower(location) == lower(input.parameters.regions[_])
}

location_allowed(location) {
  lower(location) == "global"
}`,
		Parameters: func(cfg *models.GovernanceConfig) map[string][]string {
			return map[string][]string{"regions": splitCSVList(cfg.AllowedRegions)}
		},
	},
	{
		ID:          "required-labels",
		Kind:        "GcpRequiredLabels",
		Description: "Require governance labels (e.g. data classification and owner) on all resources",
		Regimes:     []string{"gdpr", "iso27001", "nis2"},
		TargetKinds: []string{"*"},
		Rego: `package gcprequiredlabels

violation[{"msg": msg}] {
  required := input.parameters.labels[_]
  not input.review.object.metadata.labels[required]
  msg := sprintf("resource is missing required label %v", [required])
}`,
		Parameters: func(cfg *models.GovernanceConfig) map[string][]string {
			return map[string][]string{"labels": splitCSVList(cfg.RequiredLabels)}
		},
	},
	{
		ID:          "sql-no-public-ip",
		Kind:        "GcpSqlNoPublicIp",
		Description: "Deny Cloud SQL instances with public IPv4 connectivity",
		Regimes:     []string{"cis", "iso27001", "pci-dss"},
		TargetKinds: []string{"SQLInstance"},
		Rego: `package gcpsqlnopublicip

violation[{"msg": msg}] {
  input.review.object.kind == "SQLInstance"
  input.review.object.spec.settings.ipConfiguration.ipv4Enabled == true
  msg := "SQLInstance must not enable a public IPv4 address"
}`,
	},
	{
		ID:          "gke-private-cluster",
		Kind:        "GcpGkePrivateCluster",
		Description: "Require GKE clusters to run private nodes",
		Regimes:     []string{"cis", "nis2", "pci-dss"},
		TargetKinds: []string{"ContainerCluster"},
		Rego: `package gcpgkeprivatecluster

violation[{"msg": msg}] {
  input.review.object.kind == "ContainerCluster"
  not input.review.object.spec.privateClusterConfig.enablePrivateNodes == true
  msg := "ContainerCluster must enable private nodes (spec.privateClusterConfig.enablePrivateNodes)"
}`,
	},
	{
		ID:          "gke-workload-identity",
		Kind:        "GcpGkeWorkloadIdentity",
		Description: "Require Workload Identity on GKE clusters instead of node service account keys",
		Regimes:     []string{"cis", "iso27001"},
		TargetKinds: []string{"ContainerCluster"},
		Rego: `package gcpgkeworkloadidentity

violation[{"msg": msg}] {
  input.review.object.kind == "ContainerCluster"
  not input.review.object.spec.workloadIdentityConfig
  msg := "ContainerCluster must configure Workload Identity (spec.workloadIdentityConfig)"
}`,
	},
	{
		ID:          "binary-authorization",
		Kind:        "GcpBinaryAuthorization",
		Description: "Require Binary Authorization on GKE clusters (secure software supply chain)",
		Regimes:     []string{"eucra", "nis2"},
		TargetKinds: []string{"ContainerCluster"},
		Rego: `package gcpbinaryauthorization

violation[{"msg": msg}] {
  input.review.object.kind == "ContainerCluster"
  not binary_authorization_enabled
  msg := "ContainerCluster must enable Binary Authorization (spec.binaryAuthorization)"
}

binary_authorization_enabled {
  mode := input.review.object.spec.binaryAuthorization.evaluationMode
  mode != "DISABLED"
}`,
	},
	{
		ID:          "firewall-no-open-ingress",
		Kind:        "GcpFirewallNoOpenIngress",
		Description: "Deny firewall rules that allow ingress from 0.0.0.0/0",
		Regimes:     []string{"cis", "nis2", "pci-dss"},
		TargetKinds: []string{"ComputeFirewall"},
		Rego: `package gcpfirewallnoopeningress

violation[{"msg": msg}] {
  obj := input.review.object
  obj.kind == "ComputeFirewall"
  not obj.spec.direction == "EGRESS"
  obj.spec.sourceRanges[_] == "0.0.0.0/0"
  msg := "ComputeFirewall must not allow ingress from 0.0.0.0/0"
}`,
	},
	{
		ID:          "shielded-vms",
		Kind:        "GcpShieldedVms",
		Description: "Require Shielded VM secure boot on Compute Engine instances",
		Regimes:     []string{"cis", "eucra"},
		TargetKinds: []string{"ComputeInstance"},
		Rego: `package gcpshieldedvms

violation[{"msg": msg}] {
  input.review.object.kind == "ComputeInstance"
  not input.review.object.spec.shieldedInstanceConfig.enableSecureBoot == true
  msg := "ComputeInstance must enable Shielded VM secure boot"
}`,
	},
	{
		ID:          "bucket-versioning",
		Kind:        "GcpBucketVersioning",
		Description: "Require object versioning on Cloud Storage buckets (recoverability)",
		Regimes:     []string{"eucra", "iso27001", "nis2"},
		TargetKinds: []string{"StorageBucket"},
		Rego: `package gcpbucketversioning

violation[{"msg": msg}] {
  input.review.object.kind == "StorageBucket"
  not input.review.object.spec.versioning.enabled == true
  msg := "StorageBucket must enable object versioning (spec.versioning.enabled)"
}`,
	},
}

type governanceBuilder struct{}

// NewGovernanceBuilder creates the governance guardrails feature builder.
func NewGovernanceBuilder() FeatureBuilder {
	return &governanceBuilder{}
}

func (b *governanceBuilder) FeatureID() models.FeatureID {
	return models.FeatureGovernance
}

func (b *governanceBuilder) Build(config interface{}) ([]Resource, error) {
	cfg, err := toGovernanceConfig(config)
	if err != nil {
		return nil, err
	}

	selected := selectedRegimeSet(cfg.Regimes)
	policies := policiesForRegimes(selected)

	var resources []Resource

	// Enable the fleet APIs and Policy Controller (managed OPA Gatekeeper)
	// so the guardrails enforce themselves once the chart is applied.
	res, err := renderTemplate("policycontroller.yaml", governancePolicyController, cfg)
	if err != nil {
		return nil, err
	}
	resources = append(resources, Resource{Name: "policycontroller.yaml", Content: res})

	// Compliance evidence: which regimes were selected and which guardrails
	// they resolved to.
	resources = append(resources, Resource{
		Name:    "regime-manifest.yaml",
		Content: renderRegimeManifest(cfg, selected, policies),
	})

	for _, p := range policies {
		resources = append(resources, Resource{
			Name:    fmt.Sprintf("policy-%s.yaml", p.ID),
			Content: renderGovernancePolicy(p, cfg, selected),
		})
	}

	return resources, nil
}

func toGovernanceConfig(config interface{}) (*models.GovernanceConfig, error) {
	cfg, err := decodeConfig[models.GovernanceConfig]("governance-guardrails", config)
	if err != nil {
		return nil, err
	}

	if err := requireName("governance-guardrails", "projectId", cfg.ProjectID); err != nil {
		return nil, err
	}
	if len(cfg.Regimes) == 0 {
		return nil, newValidationError("governance-guardrails", "regimes",
			"at least one compliance regime is required (cis, gdpr, pci-dss, iso27001, nis2, eucra)")
	}
	for _, r := range cfg.Regimes {
		if _, known := governanceRegimes[r]; !known {
			return nil, newValidationError("governance-guardrails", "regimes",
				fmt.Sprintf("unknown regime %q (valid: cis, gdpr, pci-dss, iso27001, nis2, eucra)", r))
		}
	}

	switch cfg.EnforcementMode {
	case "":
		cfg.EnforcementMode = "deny"
	case "deny", "dryrun":
	default:
		return nil, newValidationError("governance-guardrails", "enforcementMode",
			fmt.Sprintf("must be \"deny\" or \"dryrun\", got %q", cfg.EnforcementMode))
	}

	if cfg.AllowedRegions == "" {
		cfg.AllowedRegions = "europe-west1,europe-west2,europe-west3,europe-west4,europe-north1"
	} else if err := validateCSVNames("governance-guardrails", "allowedRegions", cfg.AllowedRegions); err != nil {
		return nil, err
	}
	if cfg.RequiredLabels == "" {
		cfg.RequiredLabels = "data-classification,owner"
	} else {
		for _, l := range splitCSVList(cfg.RequiredLabels) {
			if err := validateModel("governance-guardrails", "requiredLabels", l); err != nil {
				return nil, err
			}
		}
	}

	return cfg, nil
}

func selectedRegimeSet(regimes []string) map[string]bool {
	set := make(map[string]bool, len(regimes))
	for _, r := range regimes {
		set[r] = true
	}
	return set
}

// policiesForRegimes returns catalog policies tagged with any selected
// regime, preserving catalog order for deterministic output.
func policiesForRegimes(selected map[string]bool) []governancePolicy {
	var out []governancePolicy
	for _, p := range governanceCatalog {
		for _, r := range p.Regimes {
			if selected[r] {
				out = append(out, p)
				break
			}
		}
	}
	return out
}

// matchedRegimes returns the sorted intersection of a policy's regimes with
// the user's selection — the reason this policy is in the output.
func matchedRegimes(p governancePolicy, selected map[string]bool) []string {
	var out []string
	for _, r := range p.Regimes {
		if selected[r] {
			out = append(out, r)
		}
	}
	sort.Strings(out)
	return out
}

// renderGovernancePolicy emits the ConstraintTemplate and its Constraint as
// one YAML document pair.
func renderGovernancePolicy(p governancePolicy, cfg *models.GovernanceConfig, selected map[string]bool) []byte {
	var b strings.Builder

	templateName := strings.ToLower(p.Kind)

	fmt.Fprintf(&b, `apiVersion: templates.gatekeeper.sh/v1
kind: ConstraintTemplate
metadata:
  name: %s
  labels:
    managed-by: activations-accelerator
    feature: governance-guardrails
  annotations:
    description: %q
spec:
  crd:
    spec:
      names:
        kind: %s
      validation:
        openAPIV3Schema:
          type: object
          x-kubernetes-preserve-unknown-fields: true
  targets:
    - target: admission.k8s.gatekeeper.sh
      rego: |
%s
---
apiVersion: constraints.gatekeeper.sh/v1beta1
kind: %s
metadata:
  name: %s
  labels:
    managed-by: activations-accelerator
    feature: governance-guardrails
  annotations:
    compliance.aillion.co/regimes: %q
    description: %q
spec:
  enforcementAction: %s
  match:
    kinds:
      - apiGroups: ["*"]
        kinds: [%s]
`,
		templateName, p.Description, p.Kind, indentBlock(p.Rego, 8),
		p.Kind, p.ID, strings.Join(matchedRegimes(p, selected), ","), p.Description,
		cfg.EnforcementMode, quoteJoin(p.TargetKinds))

	if p.Parameters != nil {
		b.WriteString("  parameters:\n")
		params := p.Parameters(cfg)
		keys := make([]string, 0, len(params))
		for k := range params {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(&b, "    %s:\n", k)
			for _, v := range params[k] {
				fmt.Fprintf(&b, "      - %s\n", v)
			}
		}
	}

	return []byte(b.String())
}

// renderRegimeManifest emits a ConfigMap that records the selected regimes
// and the guardrails each one resolved to — an auditable compliance mapping
// that ships inside the chart.
func renderRegimeManifest(cfg *models.GovernanceConfig, selected map[string]bool, policies []governancePolicy) []byte {
	var b strings.Builder

	b.WriteString(`apiVersion: v1
kind: ConfigMap
metadata:
  name: governance-regime-manifest
  namespace: config-connector
  labels:
    managed-by: activations-accelerator
    feature: governance-guardrails
data:
  enforcementMode: ` + cfg.EnforcementMode + "\n")

	regimes := make([]string, 0, len(selected))
	for r := range selected {
		regimes = append(regimes, r)
	}
	sort.Strings(regimes)

	b.WriteString("  regimes: |\n")
	for _, r := range regimes {
		fmt.Fprintf(&b, "    %s: %s\n", r, governanceRegimes[r])
	}

	b.WriteString("  policies: |\n")
	for _, p := range policies {
		fmt.Fprintf(&b, "    %s: covers %s\n", p.ID, strings.Join(matchedRegimes(p, selected), ", "))
	}

	return []byte(b.String())
}

func indentBlock(s string, spaces int) string {
	pad := strings.Repeat(" ", spaces)
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		if l != "" {
			lines[i] = pad + l
		}
	}
	return strings.Join(lines, "\n")
}

func quoteJoin(items []string) string {
	quoted := make([]string, len(items))
	for i, s := range items {
		quoted[i] = fmt.Sprintf("%q", s)
	}
	return strings.Join(quoted, ", ")
}

func splitCSVList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// governancePolicyController enables the fleet Policy Controller feature —
// Google-managed OPA Gatekeeper — plus its required APIs, so the guardrails
// in this chart are admission-enforced on the config cluster.
const governancePolicyController = `apiVersion: serviceusage.cnrm.cloud.google.com/v1beta1
kind: Service
metadata:
  name: {{ .ProjectID }}-gkehub
  namespace: config-connector
  annotations:
    cnrm.cloud.google.com/project-id: {{ .ProjectID }}
spec:
  resourceID: gkehub.googleapis.com
---
apiVersion: serviceusage.cnrm.cloud.google.com/v1beta1
kind: Service
metadata:
  name: {{ .ProjectID }}-anthospolicycontroller
  namespace: config-connector
  annotations:
    cnrm.cloud.google.com/project-id: {{ .ProjectID }}
spec:
  resourceID: anthospolicycontroller.googleapis.com
---
apiVersion: gkehub.cnrm.cloud.google.com/v1beta1
kind: GKEHubFeature
metadata:
  name: policycontroller
  namespace: config-connector
  labels:
    managed-by: activations-accelerator
    feature: governance-guardrails
spec:
  projectRef:
    external: projects/{{ .ProjectID }}
  location: global
  resourceID: policycontroller
`

package kcc

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aillion-co/infrastructure-lz/internal/models"
)

type bootstrapOrgBuilder struct{}

func NewBootstrapOrgBuilder() FeatureBuilder {
	return &bootstrapOrgBuilder{}
}

func (b *bootstrapOrgBuilder) FeatureID() models.FeatureID {
	return models.FeatureBootstrapOrg
}

func (b *bootstrapOrgBuilder) Build(config interface{}) ([]Resource, error) {
	cfg, err := toBootstrapOrgConfig(config)
	if err != nil {
		return nil, err
	}

	var resources []Resource

	// Management folder
	res, err := renderTemplate("mgmt-folder.yaml", bootstrapOrgMgmtFolder, cfg)
	if err != nil {
		return nil, err
	}
	resources = append(resources, Resource{Name: "mgmt-folder.yaml", Content: res})

	// Management project
	res, err = renderTemplate("mgmt-project.yaml", bootstrapOrgMgmtProject, cfg)
	if err != nil {
		return nil, err
	}
	resources = append(resources, Resource{Name: "mgmt-project.yaml", Content: res})

	// CI/CD service account
	res, err = renderTemplate("cicd-service-account.yaml", bootstrapOrgCICDServiceAccount, cfg)
	if err != nil {
		return nil, err
	}
	resources = append(resources, Resource{Name: "cicd-service-account.yaml", Content: res})

	// Org policies (if enabled and at org level)
	if cfg.OrgPolicies && cfg.RootLevel == "organization" {
		res, err = renderTemplate("org-policies.yaml", bootstrapOrgPolicies, cfg)
		if err != nil {
			return nil, err
		}
		resources = append(resources, Resource{Name: "org-policies.yaml", Content: res})
	}

	// Shared VPC host project
	if cfg.SharedVPC {
		res, err = renderTemplate("shared-vpc.yaml", bootstrapOrgSharedVPC, cfg)
		if err != nil {
			return nil, err
		}
		resources = append(resources, Resource{Name: "shared-vpc.yaml", Content: res})
	}

	// Environment projects
	res, err = renderTemplate("env-projects.yaml", bootstrapOrgEnvProjects, cfg)
	if err != nil {
		return nil, err
	}
	resources = append(resources, Resource{Name: "env-projects.yaml", Content: res})

	// GKE clusters per environment (if enabled)
	if cfg.GKECluster {
		res, err = renderTemplate("env-gke.yaml", bootstrapOrgEnvGKE, cfg)
		if err != nil {
			return nil, err
		}
		resources = append(resources, Resource{Name: "env-gke.yaml", Content: res})
	}

	// VPC Service Controls (if enabled and at org level)
	if cfg.VPCSC && cfg.RootLevel == "organization" {
		res, err = renderTemplate("vpc-sc.yaml", bootstrapOrgVPCSC, cfg)
		if err != nil {
			return nil, err
		}
		resources = append(resources, Resource{Name: "vpc-sc.yaml", Content: res})
	}

	return resources, nil
}

func toBootstrapOrgConfig(config interface{}) (*models.BootstrapOrgConfig, error) {
	cfg, ok := config.(*models.BootstrapOrgConfig)
	if !ok {
		data, err := json.Marshal(config)
		if err != nil {
			return nil, fmt.Errorf("marshalling bootstrap-org config: %w", err)
		}
		cfg = &models.BootstrapOrgConfig{}
		if err := json.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("unmarshalling bootstrap-org config: %w", err)
		}
	}
	cfg.Envs = strings.TrimSpace(cfg.Envs)
	if cfg.CustomerName == "" {
		return nil, newValidationError("bootstrap-org", "customerName", "is required")
	}
	if cfg.WorkloadName == "" {
		return nil, newValidationError("bootstrap-org", "workloadName", "is required")
	}
	if cfg.RootLevel != "organization" && cfg.RootLevel != "folder" {
		return nil, newValidationError("bootstrap-org", "rootLevel", "must be \"organization\" or \"folder\"")
	}
	if cfg.RootID == "" {
		return nil, newValidationError("bootstrap-org", "rootId", "is required")
	}
	if cfg.BillingAccount == "" {
		return nil, newValidationError("bootstrap-org", "billingAccount", "is required")
	}
	if cfg.Region == "" {
		return nil, newValidationError("bootstrap-org", "region", "is required")
	}
	if cfg.Envs == "" {
		return nil, newValidationError("bootstrap-org", "envs", "is required (provide a comma-separated list of environment names, e.g. \"dev,test,prod\")")
	}
	if cfg.OrgPolicies && cfg.RootLevel != "organization" {
		return nil, newValidationError("bootstrap-org", "orgPolicies", "requires rootLevel \"organization\"")
	}
	if cfg.VPCSC && cfg.RootLevel != "organization" {
		return nil, newValidationError("bootstrap-org", "vpcSc", "requires rootLevel \"organization\"")
	}
	if cfg.ServiceProjects && !cfg.SharedVPC {
		return nil, newValidationError("bootstrap-org", "serviceProjects", "requires sharedVpc to be enabled")
	}
	return cfg, nil
}

const bootstrapOrgMgmtFolder = `apiVersion: resourcemanager.cnrm.cloud.google.com/v1beta1
kind: Folder
metadata:
  name: {{ .CustomerName }}-mgmt
  namespace: config-connector
spec:
  displayName: {{ .CustomerName }}-management
{{- if eq .RootLevel "organization" }}
  organizationRef:
    external: {{ .RootID }}
{{- else }}
  folderRef:
    external: {{ .RootID }}
{{- end }}
`

const bootstrapOrgMgmtProject = `apiVersion: resourcemanager.cnrm.cloud.google.com/v1beta1
kind: Project
metadata:
  name: {{ .CustomerName }}-mgmt
  namespace: config-connector
  annotations:
    cnrm.cloud.google.com/auto-create-network: "false"
spec:
  name: {{ .CustomerName }}-mgmt
  folderRef:
    name: {{ .CustomerName }}-mgmt
  billingAccountRef:
    external: {{ .BillingAccount }}
---
apiVersion: serviceusage.cnrm.cloud.google.com/v1beta1
kind: Service
metadata:
  name: {{ .CustomerName }}-mgmt-compute
  namespace: config-connector
  annotations:
    cnrm.cloud.google.com/project-id: {{ .CustomerName }}-mgmt
spec:
  resourceID: compute.googleapis.com
---
apiVersion: serviceusage.cnrm.cloud.google.com/v1beta1
kind: Service
metadata:
  name: {{ .CustomerName }}-mgmt-container
  namespace: config-connector
  annotations:
    cnrm.cloud.google.com/project-id: {{ .CustomerName }}-mgmt
spec:
  resourceID: container.googleapis.com
---
apiVersion: serviceusage.cnrm.cloud.google.com/v1beta1
kind: Service
metadata:
  name: {{ .CustomerName }}-mgmt-iam
  namespace: config-connector
  annotations:
    cnrm.cloud.google.com/project-id: {{ .CustomerName }}-mgmt
spec:
  resourceID: iam.googleapis.com
---
apiVersion: serviceusage.cnrm.cloud.google.com/v1beta1
kind: Service
metadata:
  name: {{ .CustomerName }}-mgmt-cloudbuild
  namespace: config-connector
  annotations:
    cnrm.cloud.google.com/project-id: {{ .CustomerName }}-mgmt
spec:
  resourceID: cloudbuild.googleapis.com
`

const bootstrapOrgCICDServiceAccount = `apiVersion: iam.cnrm.cloud.google.com/v1beta1
kind: IAMServiceAccount
metadata:
  name: {{ .CustomerName }}-cicd
  namespace: config-connector
  annotations:
    cnrm.cloud.google.com/project-id: {{ .CustomerName }}-mgmt
spec:
  displayName: CI/CD Service Account for {{ .CustomerName }}
---
apiVersion: iam.cnrm.cloud.google.com/v1beta1
kind: IAMPolicyMember
metadata:
  name: {{ .CustomerName }}-cicd-project-creator
  namespace: config-connector
spec:
  member: serviceAccount:{{ .CustomerName }}-cicd@{{ .CustomerName }}-mgmt.iam.gserviceaccount.com
  role: roles/resourcemanager.projectCreator
{{- if eq .RootLevel "organization" }}
  resourceRef:
    apiVersion: resourcemanager.cnrm.cloud.google.com/v1beta1
    kind: Organization
    external: {{ .RootID }}
{{- else }}
  resourceRef:
    apiVersion: resourcemanager.cnrm.cloud.google.com/v1beta1
    kind: Folder
    external: {{ .RootID }}
{{- end }}
---
apiVersion: iam.cnrm.cloud.google.com/v1beta1
kind: IAMPolicyMember
metadata:
  name: {{ .CustomerName }}-cicd-billing-user
  namespace: config-connector
spec:
  member: serviceAccount:{{ .CustomerName }}-cicd@{{ .CustomerName }}-mgmt.iam.gserviceaccount.com
  role: roles/billing.user
  resourceRef:
    apiVersion: resourcemanager.cnrm.cloud.google.com/v1beta1
    kind: Project
    name: {{ .CustomerName }}-mgmt
`

const bootstrapOrgPolicies = `apiVersion: resourcemanager.cnrm.cloud.google.com/v1beta1
kind: ResourceManagerPolicy
metadata:
  name: {{ .CustomerName }}-skip-default-network
  namespace: config-connector
spec:
  organizationRef:
    external: {{ .RootID }}
  constraint: constraints/compute.skipDefaultNetworkCreation
  booleanPolicy:
    enforced: true
---
apiVersion: resourcemanager.cnrm.cloud.google.com/v1beta1
kind: ResourceManagerPolicy
metadata:
  name: {{ .CustomerName }}-require-os-login
  namespace: config-connector
spec:
  organizationRef:
    external: {{ .RootID }}
  constraint: constraints/compute.requireOsLogin
  booleanPolicy:
    enforced: true
---
apiVersion: resourcemanager.cnrm.cloud.google.com/v1beta1
kind: ResourceManagerPolicy
metadata:
  name: {{ .CustomerName }}-disable-sa-key-creation
  namespace: config-connector
spec:
  organizationRef:
    external: {{ .RootID }}
  constraint: constraints/iam.disableServiceAccountKeyCreation
  booleanPolicy:
    enforced: true
---
apiVersion: resourcemanager.cnrm.cloud.google.com/v1beta1
kind: ResourceManagerPolicy
metadata:
  name: {{ .CustomerName }}-public-access-prevention
  namespace: config-connector
spec:
  organizationRef:
    external: {{ .RootID }}
  constraint: constraints/storage.publicAccessPrevention
  booleanPolicy:
    enforced: true
---
apiVersion: resourcemanager.cnrm.cloud.google.com/v1beta1
kind: ResourceManagerPolicy
metadata:
  name: {{ .CustomerName }}-uniform-bucket-access
  namespace: config-connector
spec:
  organizationRef:
    external: {{ .RootID }}
  constraint: constraints/storage.uniformBucketLevelAccess
  booleanPolicy:
    enforced: true
---
apiVersion: resourcemanager.cnrm.cloud.google.com/v1beta1
kind: ResourceManagerPolicy
metadata:
  name: {{ .CustomerName }}-restrict-public-ip-sql
  namespace: config-connector
spec:
  organizationRef:
    external: {{ .RootID }}
  constraint: constraints/sql.restrictPublicIp
  booleanPolicy:
    enforced: true
---
apiVersion: resourcemanager.cnrm.cloud.google.com/v1beta1
kind: ResourceManagerPolicy
metadata:
  name: {{ .CustomerName }}-vm-external-ip
  namespace: config-connector
spec:
  organizationRef:
    external: {{ .RootID }}
  constraint: constraints/compute.vmExternalIpAccess
  listPolicy:
    allValues: DENY
---
apiVersion: resourcemanager.cnrm.cloud.google.com/v1beta1
kind: ResourceManagerPolicy
metadata:
  name: {{ .CustomerName }}-resource-locations
  namespace: config-connector
spec:
  organizationRef:
    external: {{ .RootID }}
  constraint: constraints/gcp.resourceLocations
  listPolicy:
    allowedValues:
      - in:{{ .Region }}
`

const bootstrapOrgSharedVPC = `apiVersion: compute.cnrm.cloud.google.com/v1beta1
kind: ComputeSharedVPCHostProject
metadata:
  name: {{ .CustomerName }}-mgmt-shared-vpc
  namespace: config-connector
spec:
  projectRef:
    name: {{ .CustomerName }}-mgmt
---
apiVersion: compute.cnrm.cloud.google.com/v1beta1
kind: ComputeNetwork
metadata:
  name: {{ .CustomerName }}-shared-vpc
  namespace: config-connector
  annotations:
    cnrm.cloud.google.com/project-id: {{ .CustomerName }}-mgmt
spec:
  routingMode: GLOBAL
  autoCreateSubnetworks: false
  deleteDefaultRoutesOnCreate: true
---
apiVersion: compute.cnrm.cloud.google.com/v1beta1
kind: ComputeSubnetwork
metadata:
  name: {{ .CustomerName }}-shared-{{ .Region }}
  namespace: config-connector
  annotations:
    cnrm.cloud.google.com/project-id: {{ .CustomerName }}-mgmt
spec:
  region: {{ .Region }}
  ipCidrRange: "10.0.0.0/20"
  networkRef:
    name: {{ .CustomerName }}-shared-vpc
  privateIpGoogleAccess: true
  secondaryIpRange:
    - rangeName: pods
      ipCidrRange: "10.64.0.0/14"
    - rangeName: services
      ipCidrRange: "10.68.0.0/20"
---
apiVersion: compute.cnrm.cloud.google.com/v1beta1
kind: ComputeRouter
metadata:
  name: {{ .CustomerName }}-shared-router
  namespace: config-connector
  annotations:
    cnrm.cloud.google.com/project-id: {{ .CustomerName }}-mgmt
spec:
  region: {{ .Region }}
  networkRef:
    name: {{ .CustomerName }}-shared-vpc
---
apiVersion: compute.cnrm.cloud.google.com/v1beta1
kind: ComputeRouterNAT
metadata:
  name: {{ .CustomerName }}-shared-nat
  namespace: config-connector
  annotations:
    cnrm.cloud.google.com/project-id: {{ .CustomerName }}-mgmt
spec:
  region: {{ .Region }}
  routerRef:
    name: {{ .CustomerName }}-shared-router
  natIpAllocateOption: AUTO_ONLY
  sourceSubnetworkIpRangesToNat: ALL_SUBNETWORKS_ALL_IP_RANGES
`

const bootstrapOrgEnvProjects = `{{- $customer := .CustomerName }}
{{- $workload := .WorkloadName }}
{{- $billing := .BillingAccount }}
{{- $region := .Region }}
{{- $envFolders := .EnvFolders }}
{{- $sharedVPC := .SharedVPC }}
{{- $serviceProjects := .ServiceProjects }}
{{- range $env := splitCSV .Envs }}
{{- if $envFolders }}
apiVersion: resourcemanager.cnrm.cloud.google.com/v1beta1
kind: Folder
metadata:
  name: {{ $customer }}-{{ $env }}
  namespace: config-connector
spec:
  displayName: {{ $customer }}-{{ $env }}
  folderRef:
    name: {{ $customer }}-mgmt
---
{{- end }}
apiVersion: resourcemanager.cnrm.cloud.google.com/v1beta1
kind: Project
metadata:
  name: {{ $customer }}-{{ $workload }}-{{ $env }}
  namespace: config-connector
  annotations:
    cnrm.cloud.google.com/auto-create-network: "false"
spec:
  name: {{ $customer }}-{{ $workload }}-{{ $env }}
{{- if $envFolders }}
  folderRef:
    name: {{ $customer }}-{{ $env }}
{{- else }}
  folderRef:
    name: {{ $customer }}-mgmt
{{- end }}
  billingAccountRef:
    external: {{ $billing }}
---
apiVersion: serviceusage.cnrm.cloud.google.com/v1beta1
kind: Service
metadata:
  name: {{ $customer }}-{{ $workload }}-{{ $env }}-compute
  namespace: config-connector
  annotations:
    cnrm.cloud.google.com/project-id: {{ $customer }}-{{ $workload }}-{{ $env }}
spec:
  resourceID: compute.googleapis.com
---
apiVersion: serviceusage.cnrm.cloud.google.com/v1beta1
kind: Service
metadata:
  name: {{ $customer }}-{{ $workload }}-{{ $env }}-container
  namespace: config-connector
  annotations:
    cnrm.cloud.google.com/project-id: {{ $customer }}-{{ $workload }}-{{ $env }}
spec:
  resourceID: container.googleapis.com
{{- if and $sharedVPC $serviceProjects }}
---
apiVersion: compute.cnrm.cloud.google.com/v1beta1
kind: ComputeSharedVPCServiceProject
metadata:
  name: {{ $customer }}-{{ $workload }}-{{ $env }}-service
  namespace: config-connector
spec:
  projectRef:
    name: {{ $customer }}-{{ $workload }}-{{ $env }}
  hostProjectRef:
    name: {{ $customer }}-mgmt
{{- end }}
---
{{ end }}
`

const bootstrapOrgEnvGKE = `{{- $customer := .CustomerName }}
{{- $workload := .WorkloadName }}
{{- $region := .Region }}
{{- range $env := splitCSV .Envs }}
apiVersion: container.cnrm.cloud.google.com/v1beta1
kind: ContainerCluster
metadata:
  name: {{ $customer }}-{{ $workload }}-{{ $env }}
  namespace: config-connector
  annotations:
    cnrm.cloud.google.com/project-id: {{ $customer }}-{{ $workload }}-{{ $env }}
spec:
  location: {{ $region }}
  enableAutopilot: true
  releaseChannel:
    channel: REGULAR
  networkRef:
    name: {{ $customer }}-shared-vpc
  subnetworkRef:
    name: {{ $customer }}-shared-{{ $region }}
  ipAllocationPolicy:
    clusterSecondaryRangeName: pods
    servicesSecondaryRangeName: services
  privateClusterConfig:
    enablePrivateNodes: true
    enablePrivateEndpoint: false
    masterIpv4CidrBlock: "172.16.0.0/28"
  workloadIdentityConfig:
    workloadPool: {{ $customer }}-{{ $workload }}-{{ $env }}.svc.id.goog
---
{{ end }}
`

const bootstrapOrgVPCSC = `apiVersion: accesscontextmanager.cnrm.cloud.google.com/v1beta1
kind: AccessContextManagerAccessPolicy
metadata:
  name: {{ .CustomerName }}-access-policy
  namespace: config-connector
spec:
  organizationRef:
    external: {{ .RootID }}
  title: {{ .CustomerName }} Access Policy
---
apiVersion: accesscontextmanager.cnrm.cloud.google.com/v1beta1
kind: AccessContextManagerServicePerimeter
metadata:
  name: {{ .CustomerName }}-perimeter
  namespace: config-connector
spec:
  accessPolicyRef:
    name: {{ .CustomerName }}-access-policy
  title: {{ .CustomerName }} Service Perimeter
  perimeterType: PERIMETER_TYPE_REGULAR
  status:
    restrictedServices:
      - storage.googleapis.com
      - bigquery.googleapis.com
      - compute.googleapis.com
`

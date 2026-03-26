package kcc

// resourceTemplates holds Go text/template strings for each KCC resource type.
// These produce YAML manifests conforming to Google Config Connector CRDs.
var resourceTemplates = map[string]string{

	"project.yaml": `apiVersion: resourcemanager.cnrm.cloud.google.com/v1beta1
kind: Project
metadata:
  name: {{ .ProjectID }}
  namespace: config-connector
  annotations:
    cnrm.cloud.google.com/auto-create-network: "false"
spec:
  name: {{ .ProjectName }}
  resourceID: {{ .ProjectID }}
`,

	"network.yaml": `apiVersion: compute.cnrm.cloud.google.com/v1beta1
kind: ComputeNetwork
metadata:
  name: {{ .Network.VPCName }}
  namespace: config-connector
  annotations:
    cnrm.cloud.google.com/project-id: {{ .ProjectID }}
spec:
  routingMode: REGIONAL
  autoCreateSubnetworks: false
  deleteDefaultRoutesOnCreate: true
`,

	"subnet.yaml": `apiVersion: compute.cnrm.cloud.google.com/v1beta1
kind: ComputeSubnetwork
metadata:
  name: {{ .Network.VPCName }}-{{ .Region }}
  namespace: config-connector
  annotations:
    cnrm.cloud.google.com/project-id: {{ .ProjectID }}
spec:
  region: {{ .Region }}
  ipCidrRange: {{ .Network.SubnetCIDR }}
  networkRef:
    name: {{ .Network.VPCName }}
  privateIpGoogleAccess: {{ .Network.EnablePrivate }}
{{- if .GKE }}
  secondaryIpRange:
    - rangeName: pods
      ipCidrRange: {{ .Network.PodCIDR }}
    - rangeName: services
      ipCidrRange: {{ .Network.ServiceCIDR }}
{{- end }}
`,

	"gke.yaml": `apiVersion: container.cnrm.cloud.google.com/v1beta1
kind: ContainerCluster
metadata:
  name: {{ .GKE.ClusterName }}
  namespace: config-connector
  annotations:
    cnrm.cloud.google.com/project-id: {{ .ProjectID }}
spec:
  location: {{ .Region }}
  initialNodeCount: 1
  removeDefaultNodePool: true
  networkRef:
    name: {{ .Network.VPCName }}
  subnetworkRef:
    name: {{ .Network.VPCName }}-{{ .Region }}
  ipAllocationPolicy:
    clusterSecondaryRangeName: pods
    servicesSecondaryRangeName: services
  privateClusterConfig:
    enablePrivateNodes: true
    enablePrivateEndpoint: false
    masterIpv4CidrBlock: "172.16.0.0/28"
  releaseChannel:
    channel: {{ .GKE.ReleaseChannel }}
  workloadIdentityConfig:
    workloadPool: {{ .ProjectID }}.svc.id.goog
---
{{- if not .GKE.EnableAutopilot }}
apiVersion: container.cnrm.cloud.google.com/v1beta1
kind: ContainerNodePool
metadata:
  name: {{ .GKE.ClusterName }}-pool
  namespace: config-connector
  annotations:
    cnrm.cloud.google.com/project-id: {{ .ProjectID }}
spec:
  location: {{ .Region }}
  clusterRef:
    name: {{ .GKE.ClusterName }}
  nodeCount: {{ .GKE.NodeCount }}
  nodeConfig:
    machineType: {{ .GKE.MachineType }}
    oauthScopes:
      - "https://www.googleapis.com/auth/cloud-platform"
    shieldedInstanceConfig:
      enableSecureBoot: true
      enableIntegrityMonitoring: true
  management:
    autoRepair: true
    autoUpgrade: true
{{- end }}
`,

	"cloudsql.yaml": `apiVersion: sql.cnrm.cloud.google.com/v1beta1
kind: SQLInstance
metadata:
  name: {{ .CloudSQL.InstanceName }}
  namespace: config-connector
  annotations:
    cnrm.cloud.google.com/project-id: {{ .ProjectID }}
spec:
  region: {{ .Region }}
  databaseVersion: {{ .CloudSQL.DatabaseType }}
  settings:
    tier: {{ .CloudSQL.Tier }}
    availabilityType: {{ if .CloudSQL.HighAvailability }}REGIONAL{{ else }}ZONAL{{ end }}
    ipConfiguration:
      ipv4Enabled: false
      privateNetworkRef:
        name: {{ .Network.VPCName }}
    backupConfiguration:
      enabled: true
      startTime: "03:00"
`,

	"iam.yaml": `{{- if .IAM.AdminGroup }}
apiVersion: iam.cnrm.cloud.google.com/v1beta1
kind: IAMPolicyMember
metadata:
  name: {{ .ProjectID }}-admin-binding
  namespace: config-connector
  annotations:
    cnrm.cloud.google.com/project-id: {{ .ProjectID }}
spec:
  member: group:{{ .IAM.AdminGroup }}
  role: roles/editor
  resourceRef:
    kind: Project
    name: {{ .ProjectID }}
{{- end }}
{{- if .IAM.ViewerGroup }}
---
apiVersion: iam.cnrm.cloud.google.com/v1beta1
kind: IAMPolicyMember
metadata:
  name: {{ .ProjectID }}-viewer-binding
  namespace: config-connector
  annotations:
    cnrm.cloud.google.com/project-id: {{ .ProjectID }}
spec:
  member: group:{{ .IAM.ViewerGroup }}
  role: roles/viewer
  resourceRef:
    kind: Project
    name: {{ .ProjectID }}
{{- end }}
`,
}

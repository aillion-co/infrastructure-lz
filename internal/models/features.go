package models

// BootstrapOrgConfig configures the foundational GCP organisation structure.
type BootstrapOrgConfig struct {
	// General
	CustomerName   string `json:"customerName"`
	WorkloadName   string `json:"workloadName"`
	RootLevel      string `json:"rootLevel"`      // "organization" or "folder"
	RootID         string `json:"rootId"`         // GCP org or folder ID
	BillingAccount string `json:"billingAccount"` // format: XXXXXX-XXXXXX-XXXXXX
	Region         string `json:"region"`
	Zone           string `json:"zone"`

	// Security
	OrgPolicies bool `json:"orgPolicies"`
	VPCSC       bool `json:"vpcSc"` // VPC Service Controls
	CMEK        bool `json:"cmek"`  // Provision a KMSKeyRing + per-purpose CryptoKeys in the mgmt project and attach them to every encryptable resource in every enabled feature.

	// Networking
	SharedVPC       bool `json:"sharedVpc"`
	ServiceProjects bool `json:"serviceProjects"`

	// Version Control
	VCS         string `json:"vcs"` // github, gitlab, bitbucket, none
	VCSOrg      string `json:"vcsOrg"`
	VCSUsername string `json:"vcsUsername"`

	// CI/CD
	Pipeline   string `json:"pipeline"` // atlantis, tfc, cloudbuild, none
	TFCOrgName string `json:"tfcOrgName,omitempty"`

	// Workloads
	Envs       string `json:"envs"` // comma-separated: "dev,test,prod"
	EnvFolders bool   `json:"envFolders"`
	GKECluster bool   `json:"gkeCluster"`
}

// BootstrapOrgOutputs are values produced by bootstrap-org that downstream features consume.
type BootstrapOrgOutputs struct {
	MgmtProjectID           string   `json:"mgmtProjectId"`
	MgmtFolderID            string   `json:"mgmtFolderId"`
	WorkloadProjectIDs      []string `json:"workloadProjectIds"`
	MgmtVPCName             string   `json:"mgmtVpcName"`
	MgmtSubnetName          string   `json:"mgmtSubnetName"`
	GKEClusterNames         []string `json:"gkeClusterNames"`
	CICDServiceAccountEmail string   `json:"cicdServiceAccountEmail"`
	TerraformStateBucket    string   `json:"terraformStateBucket"`
}

// BigQueryAnalyticsConfig configures BigQuery dataset deployment.
type BigQueryAnalyticsConfig struct {
	ProjectName             string `json:"projectName"`
	ProjectID               string `json:"projectId"`
	Region                  string `json:"region"`
	DatasetID               string `json:"datasetId"`
	DatasetDescription      string `json:"datasetDescription,omitempty"`
	DefaultTableExpMS       string `json:"defaultTableExpirationMs,omitempty"`
	DeleteContentsOnDestroy bool   `json:"deleteContentsOnDestroy"`
	EnableDataTransfer      bool   `json:"enableDataTransfer"`
	DataSourceType          string `json:"dataSourceType,omitempty"` // none, crm, google-analytics
	DataViewerGroup         string `json:"dataViewerGroup,omitempty"`
	EnableScheduledQueries  bool   `json:"enableScheduledQueries"`

	// CMEK is propagated from bootstrap-org by the activation generator; do not
	// set it by hand. When true, CMEKKeyPrefix names the KMSKeyRing created by
	// bootstrap-org and every encryptable resource gets a kmsKeyRef.
	CMEK          bool   `json:"cmek,omitempty"`
	CMEKKeyPrefix string `json:"cmekKeyPrefix,omitempty"`
}

// DeveloperPortalConfig configures the Backstage developer portal deployment.
type DeveloperPortalConfig struct {
	ProjectName  string `json:"projectName"`
	GCPProjectID string `json:"gcpProjectId"`
	GCPRegion    string `json:"gcpRegion"`
	GCPNetwork   string `json:"gcpNetwork,omitempty"`
	GCPSubnet    string `json:"gcpSubnet,omitempty"`
	GitRepoSSH   string `json:"gitRepoSshUrl"`

	CMEK          bool   `json:"cmek,omitempty"`
	CMEKKeyPrefix string `json:"cmekKeyPrefix,omitempty"`
}

// HardenedImageBakeryConfig configures the CIS-hardened image pipeline.
type HardenedImageBakeryConfig struct {
	ProjectName        string `json:"projectName"`
	ProjectID          string `json:"projectId"`
	Region             string `json:"region"`
	Zone               string `json:"zone"`
	Network            string `json:"network"`
	Subnetwork         string `json:"subnetwork"`
	BuildSA            string `json:"buildSa"`
	LogsBucket         string `json:"logsBucket,omitempty"`
	ArtifactRepository string `json:"artifactRepository,omitempty"`
	PackerVersion      string `json:"packerVersion"`
	AnsibleVersion     string `json:"ansibleVersion"`

	CMEK          bool   `json:"cmek,omitempty"`
	CMEKKeyPrefix string `json:"cmekKeyPrefix,omitempty"`
}

// SecureInferencingConfig configures the LiteLLM AI proxy deployment.
type SecureInferencingConfig struct {
	ProjectName          string `json:"projectName"`
	ProjectID            string `json:"projectId"`
	Region               string `json:"region"`
	LiteLLMImage         string `json:"litellmImage,omitempty"`
	EnableGemini         bool   `json:"enableGemini"`
	GeminiModel          string `json:"geminiModel,omitempty"`
	EnableAuditLogging   bool   `json:"enableAuditLogging"`
	EnableCostTracking   bool   `json:"enableCostTracking"`
	CloudRunCPU          string `json:"cloudRunCpu,omitempty"`
	CloudRunMemory       string `json:"cloudRunMemory,omitempty"`
	CloudRunMaxInstances int    `json:"cloudRunMaxInstances,omitempty"`
	AllowedDomains       string `json:"allowedDomains,omitempty"`

	CMEK          bool   `json:"cmek,omitempty"`
	CMEKKeyPrefix string `json:"cmekKeyPrefix,omitempty"`
}

// SkaffoldAppDevConfig configures the Kubernetes application scaffold.
type SkaffoldAppDevConfig struct {
	ProjectName string `json:"projectName"`
	ServiceName string `json:"serviceName"`
	Description string `json:"description,omitempty"`
	Authors     string `json:"authors,omitempty"`
	Region      string `json:"region"`

	// Cluster
	ClusterType       string `json:"clusterType"`     // autopilot, standard
	ClusterTopology   string `json:"clusterTopology"` // regional, zonal
	MachineType       string `json:"machineType"`
	CustomMachineType string `json:"customMachineType,omitempty"`
	DiskSizeGB        int    `json:"diskSizeGb,omitempty"`
	EnableAutoscaling bool   `json:"enableAutoscaling"`
	MinNodes          int    `json:"minNodes,omitempty"`
	MaxNodes          int    `json:"maxNodes,omitempty"`
	InitialNodeCount  int    `json:"initialNodeCount,omitempty"`
	SpotVMs           bool   `json:"spotVms"`
	ReleaseChannel    string `json:"releaseChannel"` // REGULAR, STABLE, RAPID

	// Networking
	ServicePort      int    `json:"servicePort"`
	TargetPort       int    `json:"targetPort"`
	ServiceNamespace string `json:"serviceNamespace,omitempty"`
	RepoAddress      string `json:"repoAddress,omitempty"`

	// Features
	SQLDB        string `json:"sqlDb"`        // yes, no
	AllowIngress string `json:"allowIngress"` // yes, no

	CMEK          bool   `json:"cmek,omitempty"`
	CMEKKeyPrefix string `json:"cmekKeyPrefix,omitempty"`
}

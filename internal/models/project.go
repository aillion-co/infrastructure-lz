package models

// ProjectConfig represents the user's infrastructure configuration choices.
type ProjectConfig struct {
	// Metadata
	ProjectName string `json:"projectName"`
	ProjectID   string `json:"projectID"`
	Region      string `json:"region"`
	Environment string `json:"environment"`

	// Networking
	Network   NetworkConfig   `json:"network"`

	// Compute
	GKE       *GKEConfig      `json:"gke,omitempty"`

	// Storage
	CloudSQL  *CloudSQLConfig `json:"cloudSQL,omitempty"`

	// IAM
	IAM       IAMConfig       `json:"iam"`
}

type NetworkConfig struct {
	VPCName       string   `json:"vpcName"`
	SubnetCIDR    string   `json:"subnetCIDR"`
	PodCIDR       string   `json:"podCIDR"`
	ServiceCIDR   string   `json:"serviceCIDR"`
	EnableNAT     bool     `json:"enableNAT"`
	EnablePrivate bool     `json:"enablePrivate"`
}

type GKEConfig struct {
	ClusterName    string `json:"clusterName"`
	NodeCount      int    `json:"nodeCount"`
	MachineType    string `json:"machineType"`
	EnableAutopilot bool  `json:"enableAutopilot"`
	ReleaseChannel string `json:"releaseChannel"`
}

type CloudSQLConfig struct {
	InstanceName string `json:"instanceName"`
	DatabaseType string `json:"databaseType"`
	Tier         string `json:"tier"`
	HighAvailability bool `json:"highAvailability"`
}

type IAMConfig struct {
	AdminGroup    string   `json:"adminGroup"`
	ViewerGroup   string   `json:"viewerGroup"`
	CustomRoles   []string `json:"customRoles,omitempty"`
}

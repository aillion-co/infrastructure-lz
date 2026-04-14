package models

// DiscoveryResponse captures the full discovery questionnaire for driving
// feature recommendations and SOW generation.
type DiscoveryResponse struct {
	CustomerInfo DiscoveryCustomerInfo `json:"customerInfo,omitempty"`
	Identity     DiscoveryIdentity     `json:"identity,omitempty"`
	ResourceMgmt DiscoveryResourceMgmt `json:"resourceManagement,omitempty"`
	Networking   DiscoveryNetworking   `json:"networking,omitempty"`
	Monitoring   DiscoveryMonitoring   `json:"monitoring,omitempty"`
	DataMgmt     DiscoveryDataMgmt     `json:"dataManagement,omitempty"`
	CostControl  DiscoveryCostControl  `json:"costControl,omitempty"`
	IACCICD      DiscoveryIACCICD      `json:"iacCicd,omitempty"`
	AIEnablement DiscoveryAIEnablement `json:"aiEnablement,omitempty"`
	Resilience   DiscoveryResilience   `json:"resilience,omitempty"`
	Security     DiscoverySecurity     `json:"security,omitempty"`
}

// DiscoveryCustomerInfo captures basic customer and engagement context.
type DiscoveryCustomerInfo struct {
	CustomerName        string `json:"customerName,omitempty"`
	SecurityContactName string `json:"securityContactName,omitempty"`
	DataContactName     string `json:"dataContactName,omitempty"`
	IndustryVertical    string `json:"industryVertical,omitempty"` // financial-services, public-sector, etc.
	EngagementType      string `json:"engagementType,omitempty"`   // workload-migration, greenfield-defined, etc.
}

// DiscoveryIdentity captures identity and access management details.
type DiscoveryIdentity struct {
	IdentityProvider string `json:"identityProvider,omitempty"` // AD, Entra, Google Identity, etc.
	ConsoleUsers     string `json:"consoleUsers,omitempty"`     // 0-50, 50-500, etc.
	AccessGroups     string `json:"accessGroups,omitempty"`     // groups-defined, groups-need-help, no-groups
}

// DiscoveryResourceMgmt captures org and resource management preferences.
type DiscoveryResourceMgmt struct {
	ExistingOrg       bool   `json:"existingOrg"`
	FoundationsLayout string `json:"foundationsLayout,omitempty"` // environment-driven, flexible
	NamingScheme      string `json:"namingScheme,omitempty"`      // obj-env-team-hash, etc.
}

// DiscoveryNetworking captures network topology and connectivity requirements.
type DiscoveryNetworking struct {
	VPCTopology        string `json:"vpcTopology,omitempty"`        // shared, island, ncc-hub-spoke
	Interconnect       string `json:"interconnect,omitempty"`       // none, dedicated, partner, ha-vpn, classic-vpn
	RFC1918Space       string `json:"rfc1918Space,omitempty"`       // 10.0.0.0/8, etc.
	SharedVPCSplit     string `json:"sharedVpcSplit,omitempty"`     // by-business-unit, by-environment, complex
	CentralizedIngress string `json:"centralizedIngress,omitempty"` // none, apigee, f5, global-lb, other
	NetworkAppliances  string `json:"networkAppliances,omitempty"`  // none, palo-alto, cisco, etc.
	NameResolution     string `json:"nameResolution,omitempty"`     // none, dns-forwarding, dns-peering
	NATGateway         bool   `json:"natGateway"`
	SecureWebProxy     bool   `json:"secureWebProxy"`
}

// DiscoveryMonitoring captures monitoring and logging preferences.
type DiscoveryMonitoring struct {
	MonitoringSolution string `json:"monitoringSolution,omitempty"` // google-observability, datadog, splunk, etc.
	MultiCloudLogs     bool   `json:"multiCloudLogs"`
}

// DiscoveryDataMgmt captures data management and encryption preferences.
type DiscoveryDataMgmt struct {
	EncryptionKeyMgmt string `json:"encryptionKeyManagement,omitempty"` // none, cloud-kms, cloud-kms-autokey, external-kms
}

// DiscoveryCostControl captures billing and budget preferences.
type DiscoveryCostControl struct {
	BillingVisibility   string `json:"billingVisibility,omitempty"` // centrally, by-department
	BillingExport       string `json:"billingExport,omitempty"`     // looker, no-export, other
	EnableBillingAlerts bool   `json:"enableBillingAlerts"`
	MonthlyBudget       string `json:"monthlyBudget,omitempty"`
	AlertThresholds     string `json:"alertThresholds,omitempty"`
	BillingAlertEmail   string `json:"billingAlertEmail,omitempty"`
	BillingAlertSlack   string `json:"billingAlertSlackWebhook,omitempty"`
}

// DiscoveryIACCICD captures infrastructure-as-code and CI/CD tooling preferences.
type DiscoveryIACCICD struct {
	VCS           string `json:"vcs,omitempty"`           // github-saas, github-enterprise, gitlab-saas, etc.
	BuildPipeline string `json:"buildPipeline,omitempty"` // cloud-build, atlantis, terraform-cloud, etc.
}

// DiscoveryAIEnablement captures AI and ML platform requirements.
type DiscoveryAIEnablement struct {
	EnableVertexAI    bool     `json:"enableVertexAi"`
	GeminiModels      []string `json:"geminiModels,omitempty"`
	VertexEndpoints   []string `json:"vertexEndpoints,omitempty"`
	EnableModelGarden bool     `json:"enableModelGarden"`
	EnableBQML        bool     `json:"enableBqMl"`
	BQMLModels        []string `json:"bqMlModels,omitempty"`
	EnableRAGPipeline bool     `json:"enableRagPipeline"`
	EnableLiteLLM     bool     `json:"enableLitellmProxy"`
	AIAuditLogging    bool     `json:"aiAuditLogging"`
	AIDLPIntegration  bool     `json:"aiDlpIntegration"`
}

// DiscoveryResilience captures uptime and disaster recovery requirements.
type DiscoveryResilience struct {
	UptimeSLO string `json:"uptimeSlo,omitempty"` // 99.9, 99.99, 99.999, other
	RTO       string `json:"rto,omitempty"`       // near-zero, less-10-min, less-30-min, other
	RPO       string `json:"rpo,omitempty"`       // near-zero, hourly, nightly, other
}

// DiscoverySecurity captures security posture and policy requirements.
type DiscoverySecurity struct {
	CMEK                    bool     `json:"cmek"`
	SecurityCommandCenter   string   `json:"securityCommandCenter,omitempty"` // standard, premium, enterprise, different-system
	TeamCapabilities        string   `json:"teamCapabilities,omitempty"`      // app-only, app-infra, etc.
	RegionalRestrictions    string   `json:"regionalRestrictions,omitempty"`  // none, eur-us, london-belgium, etc.
	NetworkRestrictions     []string `json:"networkRestrictions,omitempty"`   // skip-default-network, restrict-shared-vpc, etc.
	StoragePolicies         []string `json:"storagePolicies,omitempty"`
	VMSecurity              []string `json:"vmSecurity,omitempty"`
	RestrictSATokenCreation bool     `json:"restrictServiceAccountTokenCreation"`
	RestrictExternalDomains bool     `json:"restrictExternalDomains"`
}

// DiscoveryFieldMeta describes a single discovery form field for UI rendering.
type DiscoveryFieldMeta struct {
	Key         string                 `json:"key"`
	Label       string                 `json:"label"`
	Type        string                 `json:"type"` // text, select, boolean, multiselect, heading
	Required    bool                   `json:"required"`
	Options     []DiscoveryFieldOption `json:"options,omitempty"`
	VisibleWhen string                 `json:"visibleWhen,omitempty"` // conditional visibility expression
}

// DiscoveryFieldOption is a single option in a select/multiselect field.
type DiscoveryFieldOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// DiscoverySectionMeta describes a section of the discovery form for wizard rendering.
// Key is the JSON field name of the section in DiscoveryResponse (e.g. "customerInfo").
type DiscoverySectionMeta struct {
	Key         string               `json:"key"`
	Title       string               `json:"title"`
	Description string               `json:"description"`
	Fields      []DiscoveryFieldMeta `json:"fields"`
}

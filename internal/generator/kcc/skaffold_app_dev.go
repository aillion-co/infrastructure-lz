package kcc

import (
	"encoding/json"
	"fmt"

	"github.com/aillion-co/infrastructure-lz/internal/models"
)

type skaffoldAppDevBuilder struct{}

func NewSkaffoldAppDevBuilder() FeatureBuilder {
	return &skaffoldAppDevBuilder{}
}

func (b *skaffoldAppDevBuilder) FeatureID() models.FeatureID {
	return models.FeatureSkaffoldAppDev
}

func (b *skaffoldAppDevBuilder) Build(config interface{}) ([]Resource, error) {
	cfg, err := toSkaffoldAppDevConfig(config)
	if err != nil {
		return nil, err
	}

	var resources []Resource

	// GKE cluster
	res, err := renderTemplate("appdev-gke.yaml", appdevGKE, cfg)
	if err != nil {
		return nil, err
	}
	resources = append(resources, Resource{Name: "appdev-gke.yaml", Content: res})

	// Application namespace and RBAC
	res, err = renderTemplate("appdev-namespace.yaml", appdevNamespace, cfg)
	if err != nil {
		return nil, err
	}
	resources = append(resources, Resource{Name: "appdev-namespace.yaml", Content: res})

	// Network policies
	res, err = renderTemplate("appdev-network-policy.yaml", appdevNetworkPolicy, cfg)
	if err != nil {
		return nil, err
	}
	resources = append(resources, Resource{Name: "appdev-network-policy.yaml", Content: res})

	// Application deployment
	res, err = renderTemplate("appdev-deployment.yaml", appdevDeployment, cfg)
	if err != nil {
		return nil, err
	}
	resources = append(resources, Resource{Name: "appdev-deployment.yaml", Content: res})

	// Cloud SQL (if enabled)
	if cfg.SQLDB == "yes" {
		res, err = renderTemplate("appdev-cloudsql.yaml", appdevCloudSQL, cfg)
		if err != nil {
			return nil, err
		}
		resources = append(resources, Resource{Name: "appdev-cloudsql.yaml", Content: res})
	}

	// CMEK IAM bindings for container and (optionally) sqladmin service
	// agents. Skaffold resources don't declare a project-id annotation, so
	// the ServiceIdentity projectRef uses ProjectName on the assumption
	// that it matches the GCP project hosting the feature.
	if cfg.CMEK {
		res, err = renderTemplate("cmek-iam.yaml", appdevCMEKIAM, cfg)
		if err != nil {
			return nil, err
		}
		resources = append(resources, Resource{Name: "cmek-iam.yaml", Content: res})
	}

	return resources, nil
}

func toSkaffoldAppDevConfig(config interface{}) (*models.SkaffoldAppDevConfig, error) {
	cfg, ok := config.(*models.SkaffoldAppDevConfig)
	if !ok {
		data, err := json.Marshal(config)
		if err != nil {
			return nil, fmt.Errorf("marshalling skaffold-app-dev config: %w", err)
		}
		cfg = &models.SkaffoldAppDevConfig{}
		if err := json.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("unmarshalling skaffold-app-dev config: %w", err)
		}
	}
	if cfg.ProjectName == "" {
		return nil, newValidationError("skaffold-application-development", "projectName", "is required")
	}
	if cfg.ServiceName == "" {
		return nil, newValidationError("skaffold-application-development", "serviceName", "is required")
	}
	if cfg.Region == "" {
		return nil, newValidationError("skaffold-application-development", "region", "is required")
	}
	return cfg, nil
}

const appdevGKE = `apiVersion: container.cnrm.cloud.google.com/v1beta1
kind: ContainerCluster
metadata:
  name: {{ .ProjectName }}-cluster
  namespace: config-connector
spec:
  location: {{ .Region }}
{{- if eq .ClusterType "autopilot" }}
  enableAutopilot: true
{{- else }}
  initialNodeCount: 1
  removeDefaultNodePool: true
{{- end }}
  releaseChannel:
    channel: {{ default "REGULAR" .ReleaseChannel }}
  privateClusterConfig:
    enablePrivateNodes: true
    enablePrivateEndpoint: false
    masterIpv4CidrBlock: "172.16.2.0/28"
  networkPolicy:
    enabled: true
    provider: CALICO
{{- if .CMEK }}
  databaseEncryption:
    state: ENCRYPTED
    keyName: {{ .CMEKKeyPrefix }}/cryptoKeys/gke
{{- end }}
{{- if not (eq .ClusterType "autopilot") }}
---
apiVersion: container.cnrm.cloud.google.com/v1beta1
kind: ContainerNodePool
metadata:
  name: {{ .ProjectName }}-pool
  namespace: config-connector
spec:
  location: {{ .Region }}
  clusterRef:
    name: {{ .ProjectName }}-cluster
  initialNodeCount: {{ if .InitialNodeCount }}{{ .InitialNodeCount }}{{ else }}3{{ end }}
{{- if .EnableAutoscaling }}
  autoscaling:
    minNodeCount: {{ if .MinNodes }}{{ .MinNodes }}{{ else }}1{{ end }}
    maxNodeCount: {{ if .MaxNodes }}{{ .MaxNodes }}{{ else }}10{{ end }}
{{- end }}
  nodeConfig:
    machineType: {{ default "e2-standard-4" .MachineType }}
{{- if .DiskSizeGB }}
    diskSizeGb: {{ .DiskSizeGB }}
{{- end }}
{{- if .SpotVMs }}
    spot: true
{{- end }}
    oauthScopes:
      - "https://www.googleapis.com/auth/cloud-platform"
    shieldedInstanceConfig:
      enableSecureBoot: true
      enableIntegrityMonitoring: true
  management:
    autoRepair: true
    autoUpgrade: true
{{- end }}
`

const appdevNamespace = `apiVersion: v1
kind: Namespace
metadata:
  name: {{ default "default" .ServiceNamespace }}
  labels:
    app.kubernetes.io/part-of: {{ .ProjectName }}
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ .ServiceName }}
  namespace: {{ default "default" .ServiceNamespace }}
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: {{ .ServiceName }}-binding
  namespace: {{ default "default" .ServiceNamespace }}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: edit
subjects:
  - kind: ServiceAccount
    name: {{ .ServiceName }}
    namespace: {{ default "default" .ServiceNamespace }}
`

const appdevNetworkPolicy = `apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: {{ .ServiceName }}-default-deny
  namespace: {{ default "default" .ServiceNamespace }}
spec:
  podSelector: {}
  policyTypes:
    - Ingress
    - Egress
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: {{ .ServiceName }}-allow-app
  namespace: {{ default "default" .ServiceNamespace }}
spec:
  podSelector:
    matchLabels:
      app: {{ .ServiceName }}
  policyTypes:
    - Ingress
    - Egress
  ingress:
    - ports:
        - port: {{ if .TargetPort }}{{ .TargetPort }}{{ else }}8080{{ end }}
          protocol: TCP
  egress:
    - {}
`

const appdevDeployment = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .ServiceName }}
  namespace: {{ default "default" .ServiceNamespace }}
spec:
  replicas: 2
  selector:
    matchLabels:
      app: {{ .ServiceName }}
  template:
    metadata:
      labels:
        app: {{ .ServiceName }}
    spec:
      serviceAccountName: {{ .ServiceName }}
      securityContext:
        runAsNonRoot: true
        runAsUser: 10001
        fsGroup: 10001
      containers:
        - name: {{ .ServiceName }}
          image: placeholder:latest
          ports:
            - containerPort: {{ if .TargetPort }}{{ .TargetPort }}{{ else }}8080{{ end }}
          securityContext:
            allowPrivilegeEscalation: false
            readOnlyRootFilesystem: true
            capabilities:
              drop:
                - ALL
          resources:
            requests:
              cpu: 100m
              memory: 128Mi
            limits:
              cpu: 500m
              memory: 512Mi
          readinessProbe:
            httpGet:
              path: /healthz
              port: {{ if .TargetPort }}{{ .TargetPort }}{{ else }}8080{{ end }}
            initialDelaySeconds: 5
          livenessProbe:
            httpGet:
              path: /healthz
              port: {{ if .TargetPort }}{{ .TargetPort }}{{ else }}8080{{ end }}
            initialDelaySeconds: 10
---
apiVersion: v1
kind: Service
metadata:
  name: {{ .ServiceName }}
  namespace: {{ default "default" .ServiceNamespace }}
spec:
  selector:
    app: {{ .ServiceName }}
  ports:
    - port: {{ if .ServicePort }}{{ .ServicePort }}{{ else }}80{{ end }}
      targetPort: {{ if .TargetPort }}{{ .TargetPort }}{{ else }}8080{{ end }}
{{- if eq .AllowIngress "yes" }}
  type: LoadBalancer
{{- else }}
  type: ClusterIP
{{- end }}
`

const appdevCloudSQL = `apiVersion: sql.cnrm.cloud.google.com/v1beta1
kind: SQLInstance
metadata:
  name: {{ .ProjectName }}-db
  namespace: config-connector
spec:
  databaseVersion: POSTGRES_15
  region: {{ .Region }}
{{- if .CMEK }}
  encryptionKMSCryptoKeyRef:
    external: {{ .CMEKKeyPrefix }}/cryptoKeys/sql
{{- end }}
  settings:
    tier: db-custom-2-8192
    availabilityType: REGIONAL
    ipConfiguration:
      ipv4Enabled: false
      requireSsl: true
    backupConfiguration:
      enabled: true
      startTime: "03:00"
      pointInTimeRecoveryEnabled: true
    maintenanceWindow:
      day: 7
      hour: 3
---
apiVersion: sql.cnrm.cloud.google.com/v1beta1
kind: SQLDatabase
metadata:
  name: {{ .ProjectName }}-db-primary
  namespace: config-connector
spec:
  instanceRef:
    name: {{ .ProjectName }}-db
  charset: UTF8
  collation: en_US.UTF8
`

// appdevCMEKIAM binds the container service agent (always) and the
// sqladmin service agent (when SQLDB is enabled) to the respective crypto
// keys in the mgmt project.
const appdevCMEKIAM = `apiVersion: serviceusage.cnrm.cloud.google.com/v1beta1
kind: ServiceIdentity
metadata:
  name: {{ .ProjectName }}-container-identity
  namespace: config-connector
spec:
  projectRef:
    external: {{ .ProjectName }}
  resourceID: container.googleapis.com
---
apiVersion: iam.cnrm.cloud.google.com/v1beta1
kind: IAMPolicyMember
metadata:
  name: {{ .ProjectName }}-gke-cmek
  namespace: config-connector
  annotations:
    cnrm.cloud.google.com/project-id: {{ .CMEKKeyProject }}
spec:
  resourceRef:
    apiVersion: kms.cnrm.cloud.google.com/v1beta1
    kind: KMSCryptoKey
    name: {{ .CMEKKeyCustomer }}-gke
  role: roles/cloudkms.cryptoKeyEncrypterDecrypter
  memberFrom:
    serviceIdentityRef:
      name: {{ .ProjectName }}-container-identity
{{- if eq .SQLDB "yes" }}
---
apiVersion: serviceusage.cnrm.cloud.google.com/v1beta1
kind: ServiceIdentity
metadata:
  name: {{ .ProjectName }}-sqladmin-identity
  namespace: config-connector
spec:
  projectRef:
    external: {{ .ProjectName }}
  resourceID: sqladmin.googleapis.com
---
apiVersion: iam.cnrm.cloud.google.com/v1beta1
kind: IAMPolicyMember
metadata:
  name: {{ .ProjectName }}-sql-cmek
  namespace: config-connector
  annotations:
    cnrm.cloud.google.com/project-id: {{ .CMEKKeyProject }}
spec:
  resourceRef:
    apiVersion: kms.cnrm.cloud.google.com/v1beta1
    kind: KMSCryptoKey
    name: {{ .CMEKKeyCustomer }}-sql
  role: roles/cloudkms.cryptoKeyEncrypterDecrypter
  memberFrom:
    serviceIdentityRef:
      name: {{ .ProjectName }}-sqladmin-identity
{{- end }}
`

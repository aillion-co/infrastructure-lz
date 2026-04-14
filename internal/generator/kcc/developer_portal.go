package kcc

import (
	"encoding/json"
	"fmt"

	"github.com/aillion-co/infrastructure-lz/internal/models"
)

type developerPortalBuilder struct{}

func NewDeveloperPortalBuilder() FeatureBuilder {
	return &developerPortalBuilder{}
}

func (b *developerPortalBuilder) FeatureID() models.FeatureID {
	return models.FeatureDeveloperPortal
}

func (b *developerPortalBuilder) Build(config interface{}) ([]Resource, error) {
	cfg, err := toDeveloperPortalConfig(config)
	if err != nil {
		return nil, err
	}

	var resources []Resource

	// APIs
	res, err := renderTemplate("portal-apis.yaml", portalAPIs, cfg)
	if err != nil {
		return nil, err
	}
	resources = append(resources, Resource{Name: "portal-apis.yaml", Content: res})

	// Config Controller cluster
	res, err = renderTemplate("config-controller.yaml", portalConfigController, cfg)
	if err != nil {
		return nil, err
	}
	resources = append(resources, Resource{Name: "config-controller.yaml", Content: res})

	// Backstage namespace and RBAC
	res, err = renderTemplate("backstage-namespace.yaml", portalBackstageNamespace, cfg)
	if err != nil {
		return nil, err
	}
	resources = append(resources, Resource{Name: "backstage-namespace.yaml", Content: res})

	// Backstage workloads (postgres + app)
	res, err = renderTemplate("backstage-workloads.yaml", portalBackstageWorkloads, cfg)
	if err != nil {
		return nil, err
	}
	resources = append(resources, Resource{Name: "backstage-workloads.yaml", Content: res})

	// Config Sync
	if cfg.GitRepoSSH != "" {
		res, err = renderTemplate("config-sync.yaml", portalConfigSync, cfg)
		if err != nil {
			return nil, err
		}
		resources = append(resources, Resource{Name: "config-sync.yaml", Content: res})
	}

	// CMEK IAM binding for the container service agent (Config Controller GKE).
	if cfg.CMEK {
		res, err = renderTemplate("cmek-iam.yaml", portalCMEKIAM, cfg)
		if err != nil {
			return nil, err
		}
		resources = append(resources, Resource{Name: "cmek-iam.yaml", Content: res})
	}

	return resources, nil
}

func toDeveloperPortalConfig(config interface{}) (*models.DeveloperPortalConfig, error) {
	cfg, ok := config.(*models.DeveloperPortalConfig)
	if !ok {
		data, err := json.Marshal(config)
		if err != nil {
			return nil, fmt.Errorf("marshalling developer-portal config: %w", err)
		}
		cfg = &models.DeveloperPortalConfig{}
		if err := json.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("unmarshalling developer-portal config: %w", err)
		}
	}
	if cfg.ProjectName == "" {
		return nil, newValidationError("dynamic-developer-portal", "projectName", "is required")
	}
	if cfg.GCPProjectID == "" {
		return nil, newValidationError("dynamic-developer-portal", "gcpProjectId", "is required")
	}
	if cfg.GCPRegion == "" {
		return nil, newValidationError("dynamic-developer-portal", "gcpRegion", "is required")
	}
	return cfg, nil
}

const portalAPIs = `apiVersion: serviceusage.cnrm.cloud.google.com/v1beta1
kind: Service
metadata:
  name: {{ .GCPProjectID }}-krmapihosting
  namespace: config-connector
  annotations:
    cnrm.cloud.google.com/project-id: {{ .GCPProjectID }}
spec:
  resourceID: krmapihosting.googleapis.com
---
apiVersion: serviceusage.cnrm.cloud.google.com/v1beta1
kind: Service
metadata:
  name: {{ .GCPProjectID }}-container
  namespace: config-connector
  annotations:
    cnrm.cloud.google.com/project-id: {{ .GCPProjectID }}
spec:
  resourceID: container.googleapis.com
---
apiVersion: serviceusage.cnrm.cloud.google.com/v1beta1
kind: Service
metadata:
  name: {{ .GCPProjectID }}-anthos
  namespace: config-connector
  annotations:
    cnrm.cloud.google.com/project-id: {{ .GCPProjectID }}
spec:
  resourceID: anthos.googleapis.com
`

const portalConfigController = `apiVersion: container.cnrm.cloud.google.com/v1beta1
kind: ContainerCluster
metadata:
  name: {{ .ProjectName }}-config-controller
  namespace: config-connector
  annotations:
    cnrm.cloud.google.com/project-id: {{ .GCPProjectID }}
spec:
  location: {{ .GCPRegion }}
  enableAutopilot: true
  releaseChannel:
    channel: REGULAR
{{- if .GCPNetwork }}
  networkRef:
    name: {{ .GCPNetwork }}
{{- end }}
{{- if .GCPSubnet }}
  subnetworkRef:
    name: {{ .GCPSubnet }}
{{- end }}
  privateClusterConfig:
    enablePrivateNodes: true
    enablePrivateEndpoint: false
    masterIpv4CidrBlock: "172.16.1.0/28"
  workloadIdentityConfig:
    workloadPool: {{ .GCPProjectID }}.svc.id.goog
{{- if .CMEK }}
  databaseEncryption:
    state: ENCRYPTED
    keyName: {{ .CMEKKeyPrefix }}/cryptoKeys/gke
{{- end }}
  addonsConfig:
    configConnectorConfig:
      enabled: true
`

const portalBackstageNamespace = `apiVersion: v1
kind: Namespace
metadata:
  name: backstage
  labels:
    app.kubernetes.io/part-of: developer-portal
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: backstage-admin
  namespace: backstage
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: admin
subjects:
  - kind: ServiceAccount
    name: default
    namespace: backstage
`

const portalBackstageWorkloads = `apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: postgres-storage
  namespace: backstage
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 2Gi
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: postgres
  namespace: backstage
spec:
  replicas: 1
  selector:
    matchLabels:
      app: postgres
  template:
    metadata:
      labels:
        app: postgres
    spec:
      containers:
        - name: postgres
          image: postgres:15-alpine
          ports:
            - containerPort: 5432
          env:
            - name: POSTGRES_DB
              value: backstage
            - name: POSTGRES_USER
              value: backstage
            - name: POSTGRES_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: backstage-secrets
                  key: POSTGRES_PASSWORD
          volumeMounts:
            - name: data
              mountPath: /var/lib/postgresql/data
          resources:
            requests:
              cpu: 250m
              memory: 256Mi
            limits:
              cpu: 500m
              memory: 512Mi
      volumes:
        - name: data
          persistentVolumeClaim:
            claimName: postgres-storage
---
apiVersion: v1
kind: Service
metadata:
  name: postgres
  namespace: backstage
spec:
  selector:
    app: postgres
  ports:
    - port: 5432
      targetPort: 5432
`

const portalConfigSync = `apiVersion: configsync.gke.io/v1beta1
kind: RepoSync
metadata:
  name: repo-sync
  namespace: backstage
spec:
  sourceFormat: unstructured
  git:
    repo: {{ .GitRepoSSH }}
    branch: main
    dir: /config
    auth: ssh
    secretRef:
      name: git-creds
`

// portalCMEKIAM binds the container service agent in the portal project to
// the gke crypto key in the mgmt project so the Config Controller cluster's
// databaseEncryption can reconcile.
const portalCMEKIAM = `apiVersion: serviceusage.cnrm.cloud.google.com/v1beta1
kind: ServiceIdentity
metadata:
  name: {{ .ProjectName }}-container-identity
  namespace: config-connector
spec:
  projectRef:
    external: {{ .GCPProjectID }}
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
`

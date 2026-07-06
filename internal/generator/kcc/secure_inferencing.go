package kcc

import (
	"github.com/aillion-co/infrastructure-lz/internal/models"
)

type secureInferencingBuilder struct{}

func NewSecureInferencingBuilder() FeatureBuilder {
	return &secureInferencingBuilder{}
}

func (b *secureInferencingBuilder) FeatureID() models.FeatureID {
	return models.FeatureSecureInferencing
}

func (b *secureInferencingBuilder) Build(config interface{}) ([]Resource, error) {
	cfg, err := toSecureInferencingConfig(config)
	if err != nil {
		return nil, err
	}

	var resources []Resource

	// APIs
	res, err := renderTemplate("inferencing-apis.yaml", inferencingAPIs, cfg)
	if err != nil {
		return nil, err
	}
	resources = append(resources, Resource{Name: "inferencing-apis.yaml", Content: res})

	// Service account
	res, err = renderTemplate("inferencing-sa.yaml", inferencingSA, cfg)
	if err != nil {
		return nil, err
	}
	resources = append(resources, Resource{Name: "inferencing-sa.yaml", Content: res})

	// Secret for LiteLLM master key
	res, err = renderTemplate("inferencing-secret.yaml", inferencingSecret, cfg)
	if err != nil {
		return nil, err
	}
	resources = append(resources, Resource{Name: "inferencing-secret.yaml", Content: res})

	// Cloud Run service
	res, err = renderTemplate("inferencing-cloudrun.yaml", inferencingCloudRun, cfg)
	if err != nil {
		return nil, err
	}
	resources = append(resources, Resource{Name: "inferencing-cloudrun.yaml", Content: res})

	return resources, nil
}

func toSecureInferencingConfig(config interface{}) (*models.SecureInferencingConfig, error) {
	cfg, err := decodeConfig[models.SecureInferencingConfig]("secure-inferencing", config)
	if err != nil {
		return nil, err
	}
	if err := requireName("secure-inferencing", "projectId", cfg.ProjectID); err != nil {
		return nil, err
	}
	if err := requireName("secure-inferencing", "projectName", cfg.ProjectName); err != nil {
		return nil, err
	}
	if err := requireName("secure-inferencing", "region", cfg.Region); err != nil {
		return nil, err
	}
	if err := validateModel("secure-inferencing", "geminiModel", cfg.GeminiModel); err != nil {
		return nil, err
	}
	return cfg, nil
}

const inferencingAPIs = `apiVersion: serviceusage.cnrm.cloud.google.com/v1beta1
kind: Service
metadata:
  name: {{ .ProjectID }}-run
  namespace: config-connector
  annotations:
    cnrm.cloud.google.com/project-id: {{ .ProjectID }}
spec:
  resourceID: run.googleapis.com
---
apiVersion: serviceusage.cnrm.cloud.google.com/v1beta1
kind: Service
metadata:
  name: {{ .ProjectID }}-secretmanager
  namespace: config-connector
  annotations:
    cnrm.cloud.google.com/project-id: {{ .ProjectID }}
spec:
  resourceID: secretmanager.googleapis.com
{{- if .EnableGemini }}
---
apiVersion: serviceusage.cnrm.cloud.google.com/v1beta1
kind: Service
metadata:
  name: {{ .ProjectID }}-aiplatform
  namespace: config-connector
  annotations:
    cnrm.cloud.google.com/project-id: {{ .ProjectID }}
spec:
  resourceID: aiplatform.googleapis.com
{{- end }}
`

const inferencingSA = `apiVersion: iam.cnrm.cloud.google.com/v1beta1
kind: IAMServiceAccount
metadata:
  name: {{ .ProjectName }}-litellm
  namespace: config-connector
  annotations:
    cnrm.cloud.google.com/project-id: {{ .ProjectID }}
spec:
  displayName: LiteLLM Proxy Service Account
---
apiVersion: iam.cnrm.cloud.google.com/v1beta1
kind: IAMPolicyMember
metadata:
  name: {{ .ProjectName }}-litellm-secret-access
  namespace: config-connector
  annotations:
    cnrm.cloud.google.com/project-id: {{ .ProjectID }}
spec:
  member: serviceAccount:{{ .ProjectName }}-litellm@{{ .ProjectID }}.iam.gserviceaccount.com
  role: roles/secretmanager.secretAccessor
  resourceRef:
    kind: Project
    name: {{ .ProjectID }}
{{- if .EnableGemini }}
---
apiVersion: iam.cnrm.cloud.google.com/v1beta1
kind: IAMPolicyMember
metadata:
  name: {{ .ProjectName }}-litellm-vertex-user
  namespace: config-connector
  annotations:
    cnrm.cloud.google.com/project-id: {{ .ProjectID }}
spec:
  member: serviceAccount:{{ .ProjectName }}-litellm@{{ .ProjectID }}.iam.gserviceaccount.com
  role: roles/aiplatform.user
  resourceRef:
    kind: Project
    name: {{ .ProjectID }}
{{- end }}
`

const inferencingSecret = `apiVersion: secretmanager.cnrm.cloud.google.com/v1beta1
kind: SecretManagerSecret
metadata:
  name: {{ .ProjectName }}-litellm-master-key
  namespace: config-connector
  annotations:
    cnrm.cloud.google.com/project-id: {{ .ProjectID }}
spec:
  replication:
    automatic: true
---
apiVersion: secretmanager.cnrm.cloud.google.com/v1beta1
kind: SecretManagerSecretVersion
metadata:
  name: {{ .ProjectName }}-litellm-master-key-v1
  namespace: config-connector
spec:
  secretRef:
    name: {{ .ProjectName }}-litellm-master-key
  secretData:
    value: sk-change-me-after-deployment
`

const inferencingCloudRun = `apiVersion: run.cnrm.cloud.google.com/v1beta1
kind: RunService
metadata:
  name: {{ .ProjectName }}-litellm-proxy
  namespace: config-connector
  annotations:
    cnrm.cloud.google.com/project-id: {{ .ProjectID }}
spec:
  location: {{ .Region }}
  template:
    containers:
      - image: {{ yamlStr (default "ghcr.io/berriai/litellm:main-latest" .LiteLLMImage) }}
        ports:
          - containerPort: 4000
        env:
          - name: LITELLM_MASTER_KEY
            valueSource:
              secretKeyRef:
                secret: {{ .ProjectName }}-litellm-master-key
                version: "latest"
{{- if .EnableGemini }}
          - name: LITELLM_MODEL
            value: vertex_ai/{{ default "gemini-2.0-flash" .GeminiModel }}
{{- end }}
{{- if .EnableAuditLogging }}
          - name: LITELLM_LOG
            value: "true"
{{- end }}
        resources:
          limits:
            cpu: {{ yamlStr (default "2" .CloudRunCPU) }}
            memory: {{ yamlStr (default "1Gi" .CloudRunMemory) }}
        startupProbe:
          httpGet:
            path: /health
            port: 4000
          initialDelaySeconds: 10
        livenessProbe:
          httpGet:
            path: /health
            port: 4000
    scaling:
      minInstanceCount: 0
      maxInstanceCount: {{ if .CloudRunMaxInstances }}{{ .CloudRunMaxInstances }}{{ else }}5{{ end }}
    serviceAccountName: {{ .ProjectName }}-litellm@{{ .ProjectID }}.iam.gserviceaccount.com
  traffic:
    - type: TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST
      percent: 100
`

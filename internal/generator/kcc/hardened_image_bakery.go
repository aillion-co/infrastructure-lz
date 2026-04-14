package kcc

import (
	"encoding/json"
	"fmt"

	"github.com/aillion-co/infrastructure-lz/internal/models"
)

type hardenedImageBakeryBuilder struct{}

func NewHardenedImageBakeryBuilder() FeatureBuilder {
	return &hardenedImageBakeryBuilder{}
}

func (b *hardenedImageBakeryBuilder) FeatureID() models.FeatureID {
	return models.FeatureHardenedImageBakery
}

func (b *hardenedImageBakeryBuilder) Build(config interface{}) ([]Resource, error) {
	cfg, err := toHardenedImageConfig(config)
	if err != nil {
		return nil, err
	}

	var resources []Resource

	// APIs
	res, err := renderTemplate("bakery-apis.yaml", bakeryAPIs, cfg)
	if err != nil {
		return nil, err
	}
	resources = append(resources, Resource{Name: "bakery-apis.yaml", Content: res})

	// Service account
	res, err = renderTemplate("bakery-sa.yaml", bakerySA, cfg)
	if err != nil {
		return nil, err
	}
	resources = append(resources, Resource{Name: "bakery-sa.yaml", Content: res})

	// Artifact Registry
	res, err = renderTemplate("bakery-artifact-registry.yaml", bakeryArtifactRegistry, cfg)
	if err != nil {
		return nil, err
	}
	resources = append(resources, Resource{Name: "bakery-artifact-registry.yaml", Content: res})

	// Cloud Build triggers
	res, err = renderTemplate("bakery-cloudbuild.yaml", bakeryCloudBuild, cfg)
	if err != nil {
		return nil, err
	}
	resources = append(resources, Resource{Name: "bakery-cloudbuild.yaml", Content: res})

	// Logs bucket
	if cfg.LogsBucket != "" {
		res, err = renderTemplate("bakery-logs-bucket.yaml", bakeryLogsBucket, cfg)
		if err != nil {
			return nil, err
		}
		resources = append(resources, Resource{Name: "bakery-logs-bucket.yaml", Content: res})
	}

	return resources, nil
}

func toHardenedImageConfig(config interface{}) (*models.HardenedImageBakeryConfig, error) {
	cfg, ok := config.(*models.HardenedImageBakeryConfig)
	if !ok {
		data, err := json.Marshal(config)
		if err != nil {
			return nil, fmt.Errorf("marshalling hardened-image config: %w", err)
		}
		cfg = &models.HardenedImageBakeryConfig{}
		if err := json.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("unmarshalling hardened-image config: %w", err)
		}
	}
	if cfg.ProjectName == "" {
		return nil, newValidationError("hardened-image-bakery", "projectName", "is required")
	}
	if cfg.ProjectID == "" {
		return nil, newValidationError("hardened-image-bakery", "projectId", "is required")
	}
	if cfg.Region == "" {
		return nil, newValidationError("hardened-image-bakery", "region", "is required")
	}
	if cfg.Zone == "" {
		return nil, newValidationError("hardened-image-bakery", "zone", "is required")
	}
	return cfg, nil
}

const bakeryAPIs = `apiVersion: serviceusage.cnrm.cloud.google.com/v1beta1
kind: Service
metadata:
  name: {{ .ProjectID }}-cloudbuild
  namespace: config-connector
  annotations:
    cnrm.cloud.google.com/project-id: {{ .ProjectID }}
spec:
  resourceID: cloudbuild.googleapis.com
---
apiVersion: serviceusage.cnrm.cloud.google.com/v1beta1
kind: Service
metadata:
  name: {{ .ProjectID }}-compute
  namespace: config-connector
  annotations:
    cnrm.cloud.google.com/project-id: {{ .ProjectID }}
spec:
  resourceID: compute.googleapis.com
---
apiVersion: serviceusage.cnrm.cloud.google.com/v1beta1
kind: Service
metadata:
  name: {{ .ProjectID }}-artifactregistry
  namespace: config-connector
  annotations:
    cnrm.cloud.google.com/project-id: {{ .ProjectID }}
spec:
  resourceID: artifactregistry.googleapis.com
`

const bakerySA = `apiVersion: iam.cnrm.cloud.google.com/v1beta1
kind: IAMServiceAccount
metadata:
  name: {{ .ProjectName }}-{{ default "builder" .BuildSA }}
  namespace: config-connector
  annotations:
    cnrm.cloud.google.com/project-id: {{ .ProjectID }}
spec:
  displayName: Image Bakery Build Service Account
---
apiVersion: iam.cnrm.cloud.google.com/v1beta1
kind: IAMPolicyMember
metadata:
  name: {{ .ProjectName }}-builder-compute-admin
  namespace: config-connector
  annotations:
    cnrm.cloud.google.com/project-id: {{ .ProjectID }}
spec:
  member: serviceAccount:{{ .ProjectName }}-{{ default "builder" .BuildSA }}@{{ .ProjectID }}.iam.gserviceaccount.com
  role: roles/compute.instanceAdmin.v1
  resourceRef:
    kind: Project
    name: {{ .ProjectID }}
---
apiVersion: iam.cnrm.cloud.google.com/v1beta1
kind: IAMPolicyMember
metadata:
  name: {{ .ProjectName }}-builder-storage-admin
  namespace: config-connector
  annotations:
    cnrm.cloud.google.com/project-id: {{ .ProjectID }}
spec:
  member: serviceAccount:{{ .ProjectName }}-{{ default "builder" .BuildSA }}@{{ .ProjectID }}.iam.gserviceaccount.com
  role: roles/storage.admin
  resourceRef:
    kind: Project
    name: {{ .ProjectID }}
`

const bakeryArtifactRegistry = `apiVersion: artifactregistry.cnrm.cloud.google.com/v1beta1
kind: ArtifactRegistryRepository
metadata:
  name: {{ .ProjectName }}-images
  namespace: config-connector
  annotations:
    cnrm.cloud.google.com/project-id: {{ .ProjectID }}
spec:
  location: {{ .Region }}
  format: DOCKER
  description: Hardened VM image repository for {{ .ProjectName }}
{{- if .CMEK }}
  kmsKeyRef:
    external: {{ .CMEKKeyPrefix }}/cryptoKeys/artifact-registry
{{- end }}
`

const bakeryCloudBuild = `apiVersion: cloudbuild.cnrm.cloud.google.com/v1beta1
kind: CloudBuildTrigger
metadata:
  name: {{ .ProjectName }}-ubuntu-build
  namespace: config-connector
  annotations:
    cnrm.cloud.google.com/project-id: {{ .ProjectID }}
spec:
  description: Build CIS-hardened Ubuntu image
  triggerTemplate:
    branchName: main
  build:
    step:
      - name: hashicorp/packer:{{ default "1.8.3" .PackerVersion }}
        args:
          - build
          - -var
          - project_id={{ .ProjectID }}
          - -var
          - zone={{ .Zone }}
          - -var
          - network={{ default "default" .Network }}
          - -var
          - subnetwork={{ default "default" .Subnetwork }}
          - packer/template-ubuntu-base.pkr.hcl
    serviceAccount: projects/{{ .ProjectID }}/serviceAccounts/{{ .ProjectName }}-{{ default "builder" .BuildSA }}@{{ .ProjectID }}.iam.gserviceaccount.com
---
apiVersion: cloudbuild.cnrm.cloud.google.com/v1beta1
kind: CloudBuildTrigger
metadata:
  name: {{ .ProjectName }}-windows-build
  namespace: config-connector
  annotations:
    cnrm.cloud.google.com/project-id: {{ .ProjectID }}
spec:
  description: Build CIS-hardened Windows image
  triggerTemplate:
    branchName: main
  build:
    step:
      - name: hashicorp/packer:{{ default "1.8.3" .PackerVersion }}
        args:
          - build
          - -var
          - project_id={{ .ProjectID }}
          - -var
          - zone={{ .Zone }}
          - -var
          - network={{ default "default" .Network }}
          - -var
          - subnetwork={{ default "default" .Subnetwork }}
          - packer/template-windows-base.pkr.hcl
    serviceAccount: projects/{{ .ProjectID }}/serviceAccounts/{{ .ProjectName }}-{{ default "builder" .BuildSA }}@{{ .ProjectID }}.iam.gserviceaccount.com
`

const bakeryLogsBucket = `apiVersion: storage.cnrm.cloud.google.com/v1beta1
kind: StorageBucket
metadata:
  name: {{ .LogsBucket }}
  namespace: config-connector
  annotations:
    cnrm.cloud.google.com/project-id: {{ .ProjectID }}
spec:
  location: {{ .Region }}
  uniformBucketLevelAccess: true
{{- if .CMEK }}
  encryption:
    defaultKmsKeyRef:
      external: {{ .CMEKKeyPrefix }}/cryptoKeys/storage
{{- end }}
  lifecycleRule:
    - action:
        type: Delete
      condition:
        age: 90
`

package kcc

import (
	"encoding/json"
	"fmt"

	"github.com/aillion-co/infrastructure-lz/internal/models"
)

type bigqueryAnalyticsBuilder struct{}

func NewBigQueryAnalyticsBuilder() FeatureBuilder {
	return &bigqueryAnalyticsBuilder{}
}

func (b *bigqueryAnalyticsBuilder) FeatureID() models.FeatureID {
	return models.FeatureBigQueryAnalytics
}

func (b *bigqueryAnalyticsBuilder) Build(config interface{}) ([]Resource, error) {
	cfg, err := toBigQueryConfig(config)
	if err != nil {
		return nil, err
	}

	var resources []Resource

	// Enable required APIs
	res, err := renderTemplate("bq-apis.yaml", bqAPIs, cfg)
	if err != nil {
		return nil, err
	}
	resources = append(resources, Resource{Name: "bq-apis.yaml", Content: res})

	// Dataset
	res, err = renderTemplate("bq-dataset.yaml", bqDataset, cfg)
	if err != nil {
		return nil, err
	}
	resources = append(resources, Resource{Name: "bq-dataset.yaml", Content: res})

	// IAM bindings for viewer group
	if cfg.DataViewerGroup != "" {
		res, err = renderTemplate("bq-iam.yaml", bqIAM, cfg)
		if err != nil {
			return nil, err
		}
		resources = append(resources, Resource{Name: "bq-iam.yaml", Content: res})
	}

	// Data transfer service
	if cfg.EnableDataTransfer {
		res, err = renderTemplate("bq-data-transfer.yaml", bqDataTransfer, cfg)
		if err != nil {
			return nil, err
		}
		resources = append(resources, Resource{Name: "bq-data-transfer.yaml", Content: res})
	}

	return resources, nil
}

func toBigQueryConfig(config interface{}) (*models.BigQueryAnalyticsConfig, error) {
	cfg, ok := config.(*models.BigQueryAnalyticsConfig)
	if !ok {
		data, err := json.Marshal(config)
		if err != nil {
			return nil, fmt.Errorf("marshalling bigquery config: %w", err)
		}
		cfg = &models.BigQueryAnalyticsConfig{}
		if err := json.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("unmarshalling bigquery config: %w", err)
		}
	}
	if cfg.ProjectID == "" {
		return nil, newValidationError("bigquery-analytics", "projectId", "is required")
	}
	if cfg.ProjectName == "" {
		return nil, newValidationError("bigquery-analytics", "projectName", "is required")
	}
	if cfg.Region == "" {
		return nil, newValidationError("bigquery-analytics", "region", "is required")
	}
	return cfg, nil
}

const bqAPIs = `apiVersion: serviceusage.cnrm.cloud.google.com/v1beta1
kind: Service
metadata:
  name: {{ .ProjectID }}-bigquery
  namespace: config-connector
  annotations:
    cnrm.cloud.google.com/project-id: {{ .ProjectID }}
spec:
  resourceID: bigquery.googleapis.com
---
apiVersion: serviceusage.cnrm.cloud.google.com/v1beta1
kind: Service
metadata:
  name: {{ .ProjectID }}-bigquerydatatransfer
  namespace: config-connector
  annotations:
    cnrm.cloud.google.com/project-id: {{ .ProjectID }}
spec:
  resourceID: bigquerydatatransfer.googleapis.com
`

const bqDataset = `apiVersion: bigquery.cnrm.cloud.google.com/v1beta1
kind: BigQueryDataset
metadata:
  name: {{ .ProjectName }}-{{ default "analytics" .DatasetID }}
  namespace: config-connector
  annotations:
    cnrm.cloud.google.com/project-id: {{ .ProjectID }}
  labels:
    managed-by: activations-accelerator
    feature: bigquery-analytics
spec:
  # BigQuery dataset IDs only allow letters, digits, and underscores, so the
  # hyphenated Kubernetes metadata.name cannot be used as the dataset ID.
  resourceID: {{ bqID (default "analytics" .DatasetID) }}
  friendlyName: {{ .ProjectName }} Analytics Dataset
  description: {{ default "Analytics dataset" .DatasetDescription }}
  location: {{ .Region }}
{{- if .DefaultTableExpMS }}
  defaultTableExpirationMs: {{ .DefaultTableExpMS }}
{{- end }}
  access:
    - role: OWNER
      specialGroup: projectOwners
    - role: READER
      specialGroup: projectReaders
    - role: WRITER
      specialGroup: projectWriters
`

const bqIAM = `apiVersion: iam.cnrm.cloud.google.com/v1beta1
kind: IAMPolicyMember
metadata:
  name: {{ .ProjectName }}-bq-viewer
  namespace: config-connector
  annotations:
    cnrm.cloud.google.com/project-id: {{ .ProjectID }}
spec:
  member: group:{{ .DataViewerGroup }}
  role: roles/bigquery.dataViewer
  resourceRef:
    kind: Project
    name: {{ .ProjectID }}
`

const bqDataTransfer = `apiVersion: bigquery.cnrm.cloud.google.com/v1beta1
kind: BigQueryDataTransferConfig
metadata:
  name: {{ .ProjectName }}-data-transfer
  namespace: config-connector
  annotations:
    cnrm.cloud.google.com/project-id: {{ .ProjectID }}
spec:
  displayName: {{ .ProjectName }} Data Transfer
  location: {{ .Region }}
  dataSourceID: {{ default "google_cloud_storage" .DataSourceType }}
  destinationDatasetID: {{ bqID (default "analytics" .DatasetID) }}
  schedule: "every 24 hours"
`

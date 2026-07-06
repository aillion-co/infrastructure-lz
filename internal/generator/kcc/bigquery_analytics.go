package kcc

import (
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
	cfg, err := decodeConfig[models.BigQueryAnalyticsConfig]("bigquery-analytics", config)
	if err != nil {
		return nil, err
	}
	if err := requireName("bigquery-analytics", "projectId", cfg.ProjectID); err != nil {
		return nil, err
	}
	if err := requireName("bigquery-analytics", "projectName", cfg.ProjectName); err != nil {
		return nil, err
	}
	if err := requireName("bigquery-analytics", "region", cfg.Region); err != nil {
		return nil, err
	}
	if err := validateOptionalID("bigquery-analytics", "datasetId", cfg.DatasetID); err != nil {
		return nil, err
	}
	if err := validateOptionalID("bigquery-analytics", "dataSourceType", cfg.DataSourceType); err != nil {
		return nil, err
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
  description: {{ yamlStr (default "Analytics dataset" .DatasetDescription) }}
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
  member: {{ yamlStr (printf "group:%s" .DataViewerGroup) }}
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

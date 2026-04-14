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

	// CMEK post-install note for BigQuery.
	// BigQuery's CMEK service agent (`bq-<projNum>@bigquery-encryption.iam
	// .gserviceaccount.com`) cannot be resolved via KCC ServiceIdentity, so
	// emit a ConfigMap documenting the one-time manual gcloud step. The
	// configmap is harmless if forgotten but gives operators a searchable
	// artefact in the cluster when they hit a KMS permission error.
	if cfg.CMEK {
		res, err = renderTemplate("cmek-iam-notes.yaml", bqCMEKIAMNotes, cfg)
		if err != nil {
			return nil, err
		}
		resources = append(resources, Resource{Name: "cmek-iam-notes.yaml", Content: res})
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
  friendlyName: {{ .ProjectName }} Analytics Dataset
  description: {{ default "Analytics dataset" .DatasetDescription }}
  location: {{ .Region }}
{{- if .DefaultTableExpMS }}
  defaultTableExpirationMs: {{ .DefaultTableExpMS }}
{{- end }}
{{- if .CMEK }}
  defaultEncryptionConfiguration:
    kmsKeyRef:
      external: {{ .CMEKKeyPrefix }}/cryptoKeys/bigquery
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
  destinationDatasetID: {{ default "analytics" .DatasetID }}
  schedule: "every 24 hours"
`

// bqCMEKIAMNotes emits a ConfigMap whose .data.README documents the one-time
// manual step required to grant BigQuery's encryption service agent access
// to the bigquery crypto key. BigQuery's service agent
// (bq-<projNum>@bigquery-encryption.iam.gserviceaccount.com) is not
// provisioned via the generic serviceusage.generateServiceIdentity endpoint,
// so KCC's ServiceIdentity + serviceIdentityRef pattern does not work for
// BigQuery. Operators must run the documented gcloud one-liner before
// bq-dataset.yaml will reconcile successfully.
const bqCMEKIAMNotes = `apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .ProjectName }}-cmek-iam-notes
  namespace: config-connector
  annotations:
    cnrm.cloud.google.com/project-id: {{ .ProjectID }}
data:
  README: |
    BigQuery CMEK requires a one-time manual IAM binding that cannot be
    expressed via Config Connector resources. Before applying this chart
    (or immediately after, if the BigQueryDataset reconcile is stuck on a
    KMS permission error), run:

      gcloud alpha services identity create \
        --service=bigquery.googleapis.com \
        --project={{ .ProjectID }}

      gcloud kms keys add-iam-policy-binding {{ .CMEKKeyCustomer }}-bigquery \
        --location=<mgmt-region> \
        --keyring={{ .CMEKKeyCustomer }}-activation \
        --project={{ .CMEKKeyProject }} \
        --member=serviceAccount:bq-$(gcloud projects describe {{ .ProjectID }} --format='value(projectNumber)')@bigquery-encryption.iam.gserviceaccount.com \
        --role=roles/cloudkms.cryptoKeyEncrypterDecrypter

    Substitute <mgmt-region> for the region the bootstrap-org KeyRing was
    created in. This ConfigMap exists only to document the required step;
    it is safe to delete after the binding is in place.
`

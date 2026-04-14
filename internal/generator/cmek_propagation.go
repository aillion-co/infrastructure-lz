package generator

import (
	"encoding/json"
	"fmt"

	"github.com/aillion-co/infrastructure-lz/internal/models"
)

// propagateCMEK inspects the bootstrap-org feature config and, if CMEK is
// enabled, injects CMEK=true plus the computed CMEKKeyPrefix into every other
// enabled feature's config. The prefix names the KMSKeyRing that
// bootstrap-org provisions in the management project, so downstream features
// can attach kmsKeyRef entries to their encryptable KCC resources without
// needing to know the customer name / region themselves.
//
// The incoming feature configs can be either typed structs (from Go tests)
// or map[string]any (from JSON-decoded wizard payloads). We normalise to a
// map by marshalling through JSON so the downstream builders, which already
// do the same, unmarshal a single coherent shape.
func propagateCMEK(req *models.ActivationRequest) error {
	var (
		cmek         bool
		customerName string
		region       string
	)
	for _, f := range req.Features {
		if f.FeatureID != models.FeatureBootstrapOrg || !f.Enabled {
			continue
		}
		m, err := configToMap(f.Config)
		if err != nil {
			return fmt.Errorf("reading bootstrap-org config: %w", err)
		}
		cmek, _ = m["cmek"].(bool)
		customerName, _ = m["customerName"].(string)
		region, _ = m["region"].(string)
		break
	}
	if !cmek {
		return nil
	}
	if customerName == "" || region == "" {
		return fmt.Errorf("CMEK requires bootstrap-org customerName and region")
	}
	prefix := fmt.Sprintf("projects/%s-mgmt/locations/%s/keyRings/%s-activation",
		customerName, region, customerName)

	for i := range req.Features {
		f := &req.Features[i]
		if !f.Enabled || f.FeatureID == models.FeatureBootstrapOrg {
			continue
		}
		m, err := configToMap(f.Config)
		if err != nil {
			return fmt.Errorf("reading %s config: %w", f.FeatureID, err)
		}
		m["cmek"] = true
		m["cmekKeyPrefix"] = prefix
		f.Config = m
	}
	return nil
}

// configToMap normalises any feature config (typed struct, pointer, or
// already-decoded map) into a map[string]any by marshalling through JSON.
func configToMap(cfg interface{}) (map[string]any, error) {
	if cfg == nil {
		return map[string]any{}, nil
	}
	if m, ok := cfg.(map[string]any); ok {
		return m, nil
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("marshalling config: %w", err)
	}
	m := map[string]any{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("unmarshalling config: %w", err)
	}
	return m, nil
}

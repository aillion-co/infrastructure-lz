package validator

import (
	"fmt"
	"net"
	"regexp"
	"strings"

	"github.com/aillion-co/infrastructure-lz/internal/models"
)

var (
	projectIDRegex = regexp.MustCompile(`^[a-z][a-z0-9-]{4,28}[a-z0-9]$`)
	nameRegex      = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}$`)
)

// ValidateProjectConfig returns a list of validation errors. Empty means valid.
func ValidateProjectConfig(cfg *models.ProjectConfig) []string {
	var errs []string

	if cfg.ProjectName == "" {
		errs = append(errs, "projectName is required")
	}

	if !projectIDRegex.MatchString(cfg.ProjectID) {
		errs = append(errs, fmt.Sprintf("projectID %q is invalid: must be 6-30 lowercase letters, digits, hyphens", cfg.ProjectID))
	}

	validRegions := map[string]bool{
		"us-central1": true, "us-east1": true, "us-west1": true,
		"europe-west1": true, "europe-west2": true, "europe-west3": true,
		"asia-southeast1": true, "asia-east1": true,
	}
	if !validRegions[cfg.Region] {
		errs = append(errs, fmt.Sprintf("region %q is not supported", cfg.Region))
	}

	validEnvs := map[string]bool{"development": true, "staging": true, "production": true}
	if !validEnvs[cfg.Environment] {
		errs = append(errs, fmt.Sprintf("environment %q must be one of: development, staging, production", cfg.Environment))
	}

	errs = append(errs, validateNetwork(&cfg.Network)...)

	if cfg.GKE != nil {
		errs = append(errs, validateGKE(cfg.GKE)...)
	}

	if cfg.CloudSQL != nil {
		errs = append(errs, validateCloudSQL(cfg.CloudSQL)...)
	}

	return errs
}

func validateNetwork(n *models.NetworkConfig) []string {
	var errs []string

	if !nameRegex.MatchString(n.VPCName) {
		errs = append(errs, fmt.Sprintf("network.vpcName %q is invalid", n.VPCName))
	}

	if _, _, err := net.ParseCIDR(n.SubnetCIDR); err != nil {
		errs = append(errs, fmt.Sprintf("network.subnetCIDR %q is not a valid CIDR", n.SubnetCIDR))
	}

	return errs
}

func validateGKE(g *models.GKEConfig) []string {
	var errs []string

	if !nameRegex.MatchString(g.ClusterName) {
		errs = append(errs, fmt.Sprintf("gke.clusterName %q is invalid", g.ClusterName))
	}

	if g.NodeCount < 1 || g.NodeCount > 100 {
		errs = append(errs, "gke.nodeCount must be between 1 and 100")
	}

	validChannels := map[string]bool{"REGULAR": true, "STABLE": true, "RAPID": true}
	if !validChannels[strings.ToUpper(g.ReleaseChannel)] {
		errs = append(errs, fmt.Sprintf("gke.releaseChannel %q must be REGULAR, STABLE, or RAPID", g.ReleaseChannel))
	}

	return errs
}

func validateCloudSQL(c *models.CloudSQLConfig) []string {
	var errs []string

	if !nameRegex.MatchString(c.InstanceName) {
		errs = append(errs, fmt.Sprintf("cloudSQL.instanceName %q is invalid", c.InstanceName))
	}

	validDBTypes := map[string]bool{"POSTGRES_14": true, "POSTGRES_15": true, "MYSQL_8_0": true}
	if !validDBTypes[c.DatabaseType] {
		errs = append(errs, fmt.Sprintf("cloudSQL.databaseType %q is not supported", c.DatabaseType))
	}

	return errs
}

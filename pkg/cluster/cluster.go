package cluster

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/MusicDin/kubitect/pkg/app"
	"github.com/MusicDin/kubitect/pkg/cluster/interfaces"
	"github.com/MusicDin/kubitect/pkg/cluster/managers"
	"github.com/MusicDin/kubitect/pkg/cluster/provisioner"
	"github.com/MusicDin/kubitect/pkg/cluster/provisioner/terraform"
	"github.com/MusicDin/kubitect/pkg/models/config"
	"github.com/MusicDin/kubitect/pkg/models/infra"
	"github.com/MusicDin/kubitect/pkg/ui"
	"github.com/MusicDin/kubitect/pkg/utils/defaults"
	"github.com/MusicDin/kubitect/pkg/utils/file"
)

type Cluster struct {
	ClusterMeta

	NewConfigPath string

	// Configuration files
	NewConfig     *config.Config
	AppliedConfig *config.Config
	InfraConfig   *infra.Config
}

// NewCluster returns new Cluster instance with populated general fields.
// Cluster name and path are extracted from the provided configuration file.
// Previously applied configuration is also read, if cluster already exists.
func NewCluster(ctx app.AppContext, configPath string) (*Cluster, error) {
	newCfg, err := readConfig(configPath, config.Config{})
	if err != nil {
		return nil, err
	}

	c := &Cluster{
		ClusterMeta: ClusterMeta{
			AppContext: ctx,
			Local:      ctx.Local(),
		},
		NewConfig:     newCfg,
		NewConfigPath: configPath,
	}

	if err := defaults.Set(c.NewConfig); err != nil {
		return nil, fmt.Errorf("failed to set config defaults: %v", err)
	}

	if err := validateConfig(c.NewConfig); err != nil {
		ui.PrintBlockE(err...)
		return nil, fmt.Errorf("invalid configuration file")
	}

	// If the cluster is created locally, ensure its name has the "local-" prefix.
	// Otherwise, disallow the use of the "local-" prefix in cluster names.
	if ctx.Local() {
		if !strings.HasPrefix(c.NewConfig.Cluster.Name, "local-") {
			c.NewConfig.Cluster.Name = "local-" + c.NewConfig.Cluster.Name
		}
	} else if strings.HasPrefix(c.NewConfig.Cluster.Name, "local-") {
		return nil, fmt.Errorf("cluster name contains the prefix 'local', which is reserved for clusters created with the '--local' flag")
	}

	c.Name = c.NewConfig.Cluster.Name
	c.Path = filepath.Join(c.ClustersDir(), c.Name)

	return c, c.Sync()
}

// Sync ensures that cluster configuration files are up to data.
func (c *Cluster) Sync() error {
	var err error

	appliedCfg, err := readConfigIfExists(c.AppliedConfigPath(), config.Config{})
	if err != nil {
		return fmt.Errorf("failed to read previously applied configuration file: %v", err)
	}

	if c.AppliedConfig != nil {
		*c.AppliedConfig = *appliedCfg
	} else {
		c.AppliedConfig = appliedCfg
	}

	infraCfg, err := readConfigIfExists(c.InfrastructureConfigPath(), infra.Config{})
	if err != nil {
		return fmt.Errorf("failed to read infrastructure file: %v", err)
	}

	if infraCfg != nil {
		if err := validateConfig(infraCfg); err != nil {
			ui.PrintBlockE(err...)
			return fmt.Errorf("infrastructure file (produced by Terraform) is invalid")
		}

		if c.InfraConfig != nil {
			*c.InfraConfig = *infraCfg
		} else {
			c.InfraConfig = infraCfg
		}
	}

	return nil
}

// Manager returns a manager instance that is responsible for managing
// Kubernetes cluster on provisioned instances.
func (c *Cluster) Manager() interfaces.Manager {
	if c.exec != nil {
		return c.exec
	}

	switch c.NewConfig.Kubernetes.Manager {
	case config.ManagerK3s:
		c.exec = managers.NewK3sManager(
			c.Name,
			c.Path,
			c.PrivateSshKeyPath(),
			c.ConfigDir(),
			c.CacheDir(),
			c.ShareDir(),
			c.NewConfig,
			c.InfraConfig,
		)
	case config.ManagerKubespray:
		c.exec = managers.NewKubesprayManager(
			c.Name,
			c.Path,
			c.PrivateSshKeyPath(),
			c.ConfigDir(),
			c.CacheDir(),
			c.ShareDir(),
			c.NewConfig,
			c.InfraConfig,
		)
	}

	return c.exec
}

// Provisioner function returns a provisioner instance that is responsible
// for provisioning virtual infrastructure based on the configuration file
// provided as input.
func (c *Cluster) Provisioner() provisioner.Provisioner {
	if c.prov != nil {
		return c.prov
	}

	c.prov = terraform.NewTerraformProvisioner(
		c.Path,
		c.ShareDir(),
		c.ShowTerraformPlan(),
		c.NewConfig,
	)

	return c.prov
}

// ApplyNewConfig replaces currently applied config with new one.
func (c *Cluster) ApplyNewConfig() error {
	err := os.MkdirAll(path.Dir(c.AppliedConfigPath()), 0744)
	if err != nil {
		return err
	}

	return file.WriteYaml(c.NewConfig, c.AppliedConfigPath(), 0644)
}

// StoreNewConfig makes a copy of the provided (new) configuration file in
// cluster directory.
func (c *Cluster) StoreNewConfig() error {
	c.NewConfigPath = filepath.Join(c.Path, DefaultConfigDir, DefaultNewConfigFilename)

	// Ensure config directory exists.
	err := os.MkdirAll(path.Dir(c.NewConfigPath), 0744)
	if err != nil {
		return err
	}

	return file.WriteYaml(c.NewConfig, c.NewConfigPath, 0644)
}

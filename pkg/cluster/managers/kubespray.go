package managers

import (
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/MusicDin/kubitect/pkg/cluster/event"
	"github.com/MusicDin/kubitect/pkg/env"
	"github.com/MusicDin/kubitect/pkg/models/config"
	"github.com/MusicDin/kubitect/pkg/models/infra"
	"github.com/MusicDin/kubitect/pkg/tools/ansible"
	"github.com/MusicDin/kubitect/pkg/tools/git"
	"github.com/MusicDin/kubitect/pkg/tools/virtualenv"
	"github.com/MusicDin/kubitect/pkg/ui"
	"gopkg.in/yaml.v3"
)

type kubespray struct {
	common
}

func NewKubesprayManager(
	clusterName string,
	clusterPath string,
	sshPrivateKeyPath string,
	configDir string,
	cacheDir string,
	sharedDir string,
	cfg *config.Config,
	infraCfg *infra.Config,
) *kubespray {
	return &kubespray{
		common: common{
			ClusterName:       clusterName,
			ClusterPath:       clusterPath,
			SshPrivateKeyPath: sshPrivateKeyPath,
			ConfigDir:         configDir,
			CacheDir:          cacheDir,
			SharedDir:         sharedDir,
			Config:            cfg,
			InfraConfig:       infraCfg,
		},
	}
}

// Init clones Kubespray project, initializes virtual environment
// and generates Ansible hosts inventory.
func (e *kubespray) Init() error {
	url := env.ConstKubesprayUrl
	ver := env.ConstKubesprayVersion

	dst := path.Join(e.ClusterPath, "ansible", "kubespray")
	err := os.RemoveAll(dst)
	if err != nil {
		return err
	}

	// Clone repository with Kubespray playbooks.
	err = git.NewGitRepo(url).WithRef(ver).Clone(dst)
	if err != nil {
		return err
	}

	if e.Ansible == nil {
		// Virtual environment.
		reqPath := filepath.Join(e.ClusterPath, "ansible/kubespray/requirements.txt")
		venvPath := filepath.Join(e.SharedDir, "venv", "kubespray", env.ConstKubesprayVersion)
		err = virtualenv.NewVirtualEnv(venvPath, reqPath).Init()
		if err != nil {
			return fmt.Errorf("kubespray: initialize virtual environment: %v", err)
		}

		ansibleBinDir := path.Join(venvPath, "bin")
		e.Ansible = ansible.NewAnsible(ansibleBinDir, e.CacheDir)
	}

	return nil
}

// Sync regenerates required Ansible inventories and Kubespray group
// variables.
func (e *kubespray) Sync() error {
	err := e.generateInventory()
	if err != nil {
		return err
	}

	return e.generateGroupVars()
}

// Create creates a Kubernetes cluster by calling appropriate Kubespray
// playbooks.
func (e *kubespray) Create() error {
	err := e.HAProxy()
	if err != nil {
		return err
	}

	err = e.KubesprayCreate()
	if err != nil {
		return err
	}

	err = e.Finalize()
	if err != nil {
		return err
	}

	// Rewrite kubeconfig before merging to prevent accidental
	// overwrite of an existing configuration.
	err = e.rewriteKubeconfig()
	if err != nil {
		return err
	}

	if e.Config.Kubernetes.Other.MergeKubeconfig {
		err := e.mergeKubeconfig()
		if err != nil {
			// Just warn about failure, since deployment has succeeded.
			ui.Print(ui.WARN, "Failed to merge kubeconfig:", err)
		}
	}

	return nil
}

// Upgrades upgrades a Kubernetes cluster by calling appropriate Kubespray
// playbooks.
func (e *kubespray) Upgrade() error {
	err := e.KubesprayUpgrade()
	if err != nil {
		return err
	}

	err = e.Finalize()
	if err != nil {
		return err
	}

	// Rewrite kubeconfig on upgrade, because it is re-fetched
	// from the server.
	return e.rewriteKubeconfig()
}

// ScaleUp adds new nodes to the cluster.
func (e *kubespray) ScaleUp(events event.Events) error {
	events = events.FilterByAction(event.Action_ScaleUp)
	if len(events) == 0 {
		return nil
	}

	err := e.HAProxy()
	if err != nil {
		return err
	}

	return e.KubesprayScale()
}

// ScaleDown gracefully removes nodes from the cluster.
func (e *kubespray) ScaleDown(events event.Events) error {
	rmNodes, err := extractRemovedNodes(events)
	if err != nil {
		return err
	}

	if len(rmNodes) == 0 {
		// No removed nodes.
		return nil
	}

	var names []string
	for _, n := range rmNodes {
		name := fmt.Sprintf("%s-%s-%s", e.ClusterName, n.GetTypeName(), n.GetID())
		names = append(names, name)
	}

	err = e.generateGroupVars()
	if err != nil {
		return err
	}

	err = e.KubesprayRemoveNodes(names)
	if err != nil {
		return err
	}

	return e.generateInventory()
}

// generateInventory creates an Ansible inventory containing cluster nodes.
func (e *kubespray) generateInventory() error {
	nodes := struct {
		ConfigNodes config.Nodes
		InfraNodes  config.Nodes
	}{
		ConfigNodes: e.Config.Cluster.Nodes,
		InfraNodes:  e.InfraConfig.Nodes,
	}

	return NewTemplate("kubespray/inventory.yaml", nodes).Write(filepath.Join(e.ConfigDir, "nodes.yaml"))
}

// generateGroupVars creates a directory of Kubespray group variables.
func (e *kubespray) generateGroupVars() error {
	groupVarsDir := filepath.Join(e.ConfigDir, "group_vars")

	err := NewTemplate("kubespray/all.yaml", e.InfraConfig.Nodes).Write(filepath.Join(groupVarsDir, "all", "all.yml"))
	if err != nil {
		return err
	}

	err = NewTemplate("kubespray/k8s-cluster.yaml", *e.Config).Write(filepath.Join(groupVarsDir, "k8s_cluster", "k8s-cluster.yaml"))
	if err != nil {
		return err
	}

	addons, err := yaml.Marshal(e.Config.Addons.Kubespray)
	if err != nil {
		return err
	}

	addonsPath := filepath.Join(groupVarsDir, "k8s_cluster", "addons.yaml")
	err = os.WriteFile(addonsPath, addons, 0644)
	if err != nil {
		return err
	}

	err = NewTemplate("kubespray/etcd.yaml", "").Write(filepath.Join(groupVarsDir, "etcd.yaml"))
	if err != nil {
		return err
	}

	return nil
}

// rewriteKubeconfig replaces context/cluster/user in kubeconfig with the
// cluster name.
func (e *kubespray) rewriteKubeconfig() error {
	replaces := map[string]string{
		"kubernetes-admin@cluster.local": e.ClusterName,
		"kubernetes-admin":               e.ClusterName,
		"cluster.local":                  e.ClusterName,
	}

	return e.common.rewriteKubeconfig(replaces)
}

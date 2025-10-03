package common

import (
	"bytes"
	goctx "context"
	tplfiles "github.com/RejwankabirHamim/cadence-iwf-poc/script"
	"html/template"
	"strconv"

	"github.com/pkg/errors"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cu "kmodules.xyz/client-go/client"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ProviderOptions struct {
	Name          string `json:"name"`
	Credential    string `json:"credential,omitempty"`
	ClusterID     string `json:"clusterID,omitempty"`
	Project       string `json:"project,omitempty"`
	Region        string `json:"region,omitempty"`
	ResourceGroup string `json:"resourceGroup,omitempty"`
	KubeConfig    string `json:"kubeConfig,omitempty"`
}

type BasicInfo struct {
	Name           string `json:"name"`
	DisplayName    string `json:"displayName"`
	OwnerID        int64  `json:"ownerID,omitempty"`
	ManagerID      int64  `json:"managerID,omitempty"`
	UserID         int64  `json:"userID,omitempty"`
	ClusterUID     string `json:"clusterUID,omitempty"`
	HubClusterID   string `json:"hubClusterID,omitempty"`
	InfraNamespace string `json:"infraNamespace,omitempty"`
}

type ImportOptions struct {
	BasicInfo BasicInfo       `json:"basicInfo,omitempty"`
	Provider  ProviderOptions `json:"provider,omitempty"`
}

type MachinePool struct {
	MachineType  string `json:"machineType"`
	MachineCount int    `json:"machineCount"`
	CPU          int    `json:"cpu"`
	Memory       int    `json:"memory"`
}
type CAPIClusterConfig struct {
	ClusterName       string        `json:"clusterName,omitempty"`
	Region            string        `json:"region,omitempty"`
	NetworkCIDR       string        `json:"networkCIDR,omitempty"`
	KubernetesVersion string        `json:"kubernetesVersion,omitempty"`
	GoogleProjectID   string        `json:"googleProjectID,omitempty"`
	ControlPlane      *MachinePool  `json:"controlPlane,omitempty"`
	WorkerPools       []MachinePool `json:"workerPools,omitempty"`
}
type ClusterProvisionConfig struct {
	CAPIClusterConfig CAPIClusterConfig `json:"capiClusterConfig"`
	ImportOptions     ImportOptions     `json:"importOptions"`
}

type KubeVirtCreateOperation struct {
	KubeVirtCredential *KubeVirtCredential
	CAPIConfig         *CAPIClusterConfig
	ImportOption       ImportOptions
}

func (opt KubeVirtCreateOperation) GetBaseImage(ctx goctx.Context, kc client.Client) (string, error) {
	return "", nil
}

func (opt KubeVirtCreateOperation) CreateScriptSecret(ctx goctx.Context, kc client.Client, scriptSecretName string) error {
	var scriptTemplate *template.Template
	var err error
	if opt.CAPIConfig.ControlPlane == nil {
		scriptTemplate, err = template.ParseFS(tplfiles.FS, "capi/kubevirt-kamaji-create.sh")
	} else {
		scriptTemplate, err = template.ParseFS(tplfiles.FS, "capi/kubevirt-create.sh")
	}
	if err != nil {
		return err
	}
	scriptData := map[string]interface{}{
		"cluster_name":           opt.CAPIConfig.ClusterName,
		"cluster_namespace":      scriptSecretName,
		"capk_guest_k8s_version": opt.CAPIConfig.KubernetesVersion,

		"worker_machine_count":  strconv.Itoa(opt.CAPIConfig.WorkerPools[0].MachineCount),
		"worker_machine_cpu":    strconv.Itoa(opt.CAPIConfig.WorkerPools[0].CPU),
		"worker_machine_memory": strconv.Itoa(opt.CAPIConfig.WorkerPools[0].Memory),

		"admin_cluster_kubeconfig_string": opt.KubeVirtCredential.KubeConfig,
	}

	if opt.CAPIConfig.ControlPlane != nil {
		scriptData["controlplane_machine_count"] = opt.CAPIConfig.ControlPlane.MachineCount
		scriptData["controlplane_machine_cpu"] = opt.CAPIConfig.ControlPlane.CPU
		scriptData["controlplane_machine_memory"] = opt.CAPIConfig.ControlPlane.Memory
	}

	var script bytes.Buffer
	err = scriptTemplate.Execute(&script, scriptData)
	if err != nil {
		return errors.Wrapf(err, "error in script template")
	}
	err = createScriptSecret(ctx, kc, script.String(), scriptSecretName, scriptSecretName)
	if err != nil {
		return errors.Wrapf(err, "failed to create or update script secret")
	}
	return nil
}

func (opt KubeVirtCreateOperation) GetCAPIConfig() *CAPIClusterConfig {
	return opt.CAPIConfig
}

func createScriptSecret(ctx goctx.Context, kc client.Client, script, scriptName, scriptNamespace string) error {
	secret := &core.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      scriptName,
			Namespace: scriptNamespace,
		},
		Type: core.SecretTypeOpaque,
		StringData: map[string]string{
			"script.sh": script,
		},
	}
	_, err := cu.CreateOrPatch(ctx, kc, secret, func(obj client.Object, createOp bool) client.Object {
		sec := obj.(*core.Secret)
		sec.Data = secret.Data
		return sec
	})
	return err
}

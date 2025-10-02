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

	presetlib "go.bytebuilders.dev/cluster-presets/pkg/cloudproviders/utils"
	"go.bytebuilders.dev/resource-model/apis/cloud/v1alpha1"
	clustermodel "go.bytebuilders.dev/resource-model/apis/cluster"
	configv1alpha1 "go.bytebuilders.dev/resource-model/apis/config/v1alpha1"
)

type ClusterProvisionConfig struct {
	CAPIClusterConfig configv1alpha1.CAPIClusterConfig `json:"capiClusterConfig"`
	ImportOptions     clustermodel.ImportOptions       `json:"importOptions"`
}

type KubeVirtCreateOperation struct {
	KubeVirtCredential *v1alpha1.KubeVirtCredential
	CAPIConfig         *configv1alpha1.CAPIClusterConfig
	ImportOption       clustermodel.ImportOptions
}

func (opt KubeVirtCreateOperation) GetBaseImage(ctx goctx.Context, kc client.Client) (string, error) {
	capiVersion, err := presetlib.GetCAPIVersionInfo(ctx, kc, opt.GetCAPIConfig().KubernetesVersion)
	if err != nil {
		return "", err
	}
	return capiVersion.Spec.CAPK.DeployerImage, nil
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
	capiVersion, err := presetlib.GetCAPIVersionInfo(ctx, kc, opt.GetCAPIConfig().KubernetesVersion)
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

		scriptData["gateway_api_version"] = capiVersion.Spec.CAPK.GatewayAPIVersion
		scriptData["cert_manager_version"] = capiVersion.Spec.CAPK.CertManagerVersion
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

func (opt KubeVirtCreateOperation) GetCAPIConfig() *configv1alpha1.CAPIClusterConfig {
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

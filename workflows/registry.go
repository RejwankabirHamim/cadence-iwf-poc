package workflows

import (
	cluster "github.com/RejwankabirHamim/cadence-iwf-poc/workflows/kubevirt"
	"github.com/RejwankabirHamim/cadence-iwf-poc/workflows/service"
	"github.com/indeedeng/iwf-golang-sdk/iwf"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

var registry = iwf.NewRegistry()

func init() {
	cfg, err := config.GetConfig() // uses $HOME/.kube/config by default; set KUBECONFIG env for custom path
	if err != nil {
		panic("failed to get kubeconfig: " + err.Error())
	}
	k8sClient, err := client.New(cfg, client.Options{})
	if err != nil {
		panic("failed to create k8s client: " + err.Error())
	}
	svc := service.NewMyService(k8sClient)

	err = registry.AddWorkflows(
		cluster.NewKubevirtWorkflow(svc),
	)
	if err != nil {
		panic(err)
	}
}

func GetRegistry() iwf.Registry {
	return registry
}

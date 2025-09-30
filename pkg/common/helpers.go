package common

import (
	goctx "context"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"log"
	"sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

var scheme = runtime.NewScheme()

func GetCAPIKubevirtKubeconfig(ctx goctx.Context, kubeconfig string, namespacedName types.NamespacedName) (string, error) {
	kubeconfigBytes := []byte(kubeconfig)
	apiConfig, err := clientcmd.Load(kubeconfigBytes)
	if err != nil {
		return "", err
	}
	restConfig, err := getRestConfig(apiConfig)
	if err != nil {
		return "", err
	}

	kc, err := GetNewRuntimeClient(restConfig)
	if err != nil {
		return "", err
	}
	configSecret, err := WaitForSecretToBeCreated(kc, namespacedName)
	if err != nil {
		return "", err
	}
	configData := configSecret.Data["value"]
	workloadKubeconfig, err := clientcmd.Load(configData)
	if err != nil {
		return "", err
	}

	CAPIKubeconfig, err := clientcmd.Write(*workloadKubeconfig)
	return string(CAPIKubeconfig), err
}

func getRestConfig(apiConfig *clientcmdapi.Config) (*rest.Config, error) {
	if apiConfig == nil {
		return controllerruntime.GetConfig()
	}
	return GenerateRestConfig(apiConfig)
}

func GenerateRestConfig(apiConfig *api.Config) (*rest.Config, error) {
	clientConfig := clientcmd.NewNonInteractiveClientConfig(*apiConfig, apiConfig.CurrentContext, &clientcmd.ConfigOverrides{}, nil)
	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, err
	}

	return restConfig, nil
}

func GetNewRuntimeClient(restConfig *rest.Config) (client.Client, error) {
	hc, err := rest.HTTPClientFor(restConfig)
	if err != nil {
		return nil, err
	}
	mapper, err := apiutil.NewDynamicRESTMapper(restConfig, hc)
	if err != nil {
		return nil, err
	}

	return client.New(restConfig, client.Options{
		Scheme: scheme,
		Mapper: mapper,
	})
}

func WaitForSecretToBeCreated(kc runtimeclient.Client, namespacedName types.NamespacedName) (*corev1.Secret, error) {
	secret := corev1.Secret{}
	err := wait.PollUntilContextTimeout(goctx.Background(), pullInterval, waitTimeout, true, func(ctx goctx.Context) (done bool, err error) {
		log.Printf("Waiting for secret to be created...")
		if err := kc.Get(ctx, namespacedName, &secret, &client.GetOptions{}); err != nil && !errors.IsNotFound(err) {
			return false, err
		}
		if err != nil {
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return nil, err
	}
	return &secret, nil
}

package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/RejwankabirHamim/cadence-iwf-poc/pkg/common"
	"github.com/RejwankabirHamim/cadence-iwf-poc/workflows"
	"github.com/RejwankabirHamim/cadence-iwf-poc/workflows/kubevirt"
	"github.com/indeedeng/iwf-golang-sdk/iwf"
	"github.com/urfave/cli"
	"go.bytebuilders.dev/resource-model/apis/cloud/v1alpha1"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	clustermodel "go.bytebuilders.dev/resource-model/apis/cluster"
)

const providerKubevirt = "kubevirt"

var client = iwf.NewClient(workflows.GetRegistry(), nil)

func BuildCApiCLI() *cli.App {
	app := cli.NewApp()
	app.Name = "iwf-api"
	app.Usage = "API to start KubeVirt workflows"

	app.Commands = []cli.Command{
		{
			Name:   "serve",
			Usage:  "Start API server",
			Action: StartAPIServer,
		},
	}
	return app
}

func StartAPIServer(c *cli.Context) {
	r := gin.Default()

	r.POST("/api/v1/clouds/:owner/:provider/cluster", ProvisionClusterHandler)

	log.Println("API server running on :8080")
	if err := r.Run(":8080"); err != nil {
		log.Fatalf("Failed to start API server: %v", err)
	}
}

func ProvisionClusterHandler(c *gin.Context) {
	cloudProvider := c.Param("provider")
	if cloudProvider != providerKubevirt {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported provider"})
		return
	}
	var params common.ClusterProvisionConfig
	if err := c.ShouldBindJSON(&params); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var cred *v1alpha1.CredentialSpec

	providerOpts, err := ProvisionCAPICluster(c.Request.Context(), cred, params, cloudProvider)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, providerOpts)
}

func ProvisionCAPICluster(
	ctx context.Context,
	cred *v1alpha1.CredentialSpec,
	params common.ClusterProvisionConfig,
	providerName string,
) (*clustermodel.ProviderOptions, error) {
	fmt.Printf("Provision capi cluster params: %+v\n", params)

	providerOptions := clustermodel.ProviderOptions{}
	providerOptions.Name = strings.ToUpper(providerName)
	providerOptions.Region = params.CAPIClusterConfig.Region
	providerOptions.ClusterID = params.CAPIClusterConfig.ClusterName

	title := fmt.Sprintf("Create Cluster `%s`", params.CAPIClusterConfig.ClusterName)

	switch providerName {
	case providerKubevirt:
		clusterOp := common.KubeVirtCreateOperation{
			KubeVirtCredential: cred.KubeVirt,
			CAPIConfig:         &params.CAPIClusterConfig,
			ImportOption:       params.ImportOptions,
		}

		workflowID := fmt.Sprintf("kubevirt-%s", time.Now().Unix())

		runID, err := client.StartWorkflow(
			ctx,
			kubevirt.KubevirtWorkflow{},
			workflowID,
			3600,
			clusterOp,
			nil,
		)
		if err != nil {
			return nil, err
		}

		log.Printf("Started workflow %s (runId=%s) for %s", workflowID, runID, title)

	default:
		return nil, errors.New("invalid provider")
	}

	return &providerOptions, nil
}

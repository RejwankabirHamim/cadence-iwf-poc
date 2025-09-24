package kubevirt

import (
	"fmt"
	"github.com/RejwankabirHamim/cadence-iwf-poc/pkg/common"
	"github.com/RejwankabirHamim/cadence-iwf-poc/workflows/service"
	"github.com/go-logr/logr"
	"github.com/indeedeng/iwf-golang-sdk/iwf"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	controllerruntime "sigs.k8s.io/controller-runtime"
)

func NewKubevirtWorkflow(svc service.MyService) iwf.ObjectWorkflow {

	return &KubevirtWorkflow{
		svc: svc,
	}
}

type KubevirtWorkflow struct {
	iwf.WorkflowDefaults

	svc service.MyService
}

func (e KubevirtWorkflow) GetWorkflowStates() []iwf.StateDef {
	return []iwf.StateDef{
		iwf.StartingStateDef(&createNamespaceState{svc: e.svc}),
		iwf.NonStartingStateDef(&createJobState{svc: e.svc}),
		iwf.NonStartingStateDef(&clusterOperationSuccessfulCheckState{svc: e.svc}),
		iwf.NonStartingStateDef(&syncCredentialState{svc: e.svc}),
		iwf.NonStartingStateDef(&cleanupNamespaceState{svc: e.svc}),
	}
}

type createNamespaceState struct {
	iwf.WorkflowStateDefaultsNoWaitUntil
	svc service.MyService
}

func (i createNamespaceState) Execute(
	ctx iwf.WorkflowContext, input iwf.Object, commandResults iwf.CommandResults, persistence iwf.Persistence,
	communication iwf.Communication,
) (*iwf.StateDecision, error) {
	var operation common.KubeVirtCreateOperation
	input.Get(&operation)
	nsname := fmt.Sprintf("%s-%s", operation.CAPIConfig.ClusterName, rand.String(6))

	logger := logr.FromContextOrDiscard(ctx)
	logger.Info(fmt.Sprintf("Creating Namespace: (%s)", nsname))

	err := i.svc.CreateNamespace(ctx, nsname)
	if err != nil {
		return nil, err
	}
	persistence.SetDataAttribute("nsname", nsname)
	persistence.SetDataAttribute("operation", operation)
	return iwf.SingleNextState(&createJobState{svc: i.svc}, input), nil
}

type createJobState struct {
	iwf.WorkflowStateDefaultsNoWaitUntil
	svc service.MyService
}

func (i createJobState) Execute(
	ctx iwf.WorkflowContext, input iwf.Object, commandResults iwf.CommandResults, persistence iwf.Persistence,
	communication iwf.Communication,
) (*iwf.StateDecision, error) {
	logger := logr.FromContextOrDiscard(ctx)
	logger.Info("Creating Job To Run Cluster Creation Script")

	var nsname string
	persistence.GetDataAttribute("nsname", &nsname)

	var operation common.KubeVirtCreateOperation
	input.Get(&operation)
	err := i.svc.CreateJob(ctx, operation, nsname)
	if err != nil {
		return nil, err
	}
	return iwf.SingleNextState(&clusterOperationSuccessfulCheckState{svc: i.svc}, input), nil
}

type clusterOperationSuccessfulCheckState struct {
	iwf.WorkflowStateDefaultsNoWaitUntil
	svc service.MyService
}

func (i clusterOperationSuccessfulCheckState) Execute(
	ctx iwf.WorkflowContext, input iwf.Object, commandResults iwf.CommandResults, persistence iwf.Persistence,
	communication iwf.Communication,
) (*iwf.StateDecision, error) {
	logger := logr.FromContextOrDiscard(ctx)
	logger.Info("Waiting for Cluster Creation Job to be Successful")

	var nsname string
	persistence.GetDataAttribute("nsname", &nsname)

	var operation common.KubeVirtCreateOperation
	input.Get(&operation)

	if err := i.svc.IsClusterOperationSuccessful(ctx, nsname); err != nil {
		logger.Error(err, "failed to create cluster")
		return nil, err
	}

	logger.Info("Successfully Created Cluster")
	return iwf.SingleNextState(&syncCredentialState{svc: i.svc}, input), nil
}

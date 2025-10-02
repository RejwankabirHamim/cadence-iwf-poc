package kubevirt

import (
	"fmt"
	"github.com/RejwankabirHamim/cadence-iwf-poc/internal/persistence"
	"github.com/RejwankabirHamim/cadence-iwf-poc/pkg/common"
	"github.com/RejwankabirHamim/cadence-iwf-poc/workflows/service"
	"github.com/go-logr/logr"
	"github.com/indeedeng/iwf-golang-sdk/iwf"
	"k8s.io/apimachinery/pkg/util/rand"
	"time"
)

func NewKubevirtWorkflow(svc service.ClusterCreateService) iwf.ObjectWorkflow {
	return &KubevirtWorkflow{
		svc: svc,
	}
}

type KubevirtWorkflow struct {
	iwf.WorkflowDefaults
	svc service.ClusterCreateService
}

func (w KubevirtWorkflow) GetPersistenceSchema() []iwf.PersistenceFieldDef {
	return []iwf.PersistenceFieldDef{
		iwf.DataAttributeDef("nsname"),
		iwf.DataAttributeDef("cleanup_reason"),
	}
}

func (e KubevirtWorkflow) GetWorkflowStates() []iwf.StateDef {
	return []iwf.StateDef{
		iwf.StartingStateDef(&createNamespaceState{svc: e.svc}),
		iwf.NonStartingStateDef(&createJobState{svc: e.svc}),
		iwf.NonStartingStateDef(&clusterOperationSuccessfulCheckState{svc: e.svc}),
		iwf.NonStartingStateDef(&syncCredentialState{svc: e.svc}),
		iwf.NonStartingStateDef(&cleanupNamespaceState{svc: e.svc}),
		iwf.NonStartingStateDef(&waitForeverState{}),
	}
}

func reportStateStatus(ctx iwf.WorkflowContext, stateName string, status string, data map[string]interface{}) {
	workflowID := ctx.GetWorkflowId()
	persistence.Save(workflowID, persistence.StateStatus{
		WorkflowID: workflowID,
		StateName:  stateName,
		Status:     status,
		Data:       data,
	})
}

type createNamespaceState struct {
	iwf.WorkflowStateDefaultsNoWaitUntil
	svc service.ClusterCreateService
}

func (i createNamespaceState) Execute(
	ctx iwf.WorkflowContext,
	input iwf.Object,
	commandResults iwf.CommandResults,
	persistence iwf.Persistence,
	communication iwf.Communication,
) (*iwf.StateDecision, error) {
	var operation common.KubeVirtCreateOperation
	input.Get(&operation)
	nsname := fmt.Sprintf("%s-%s", operation.CAPIConfig.ClusterName, rand.String(6))

	logger := logr.FromContextOrDiscard(ctx)
	logger.Info(fmt.Sprintf("Creating Namespace: (%s)", nsname))

	if err := i.svc.CreateNamespace(ctx, nsname); err != nil {
		reportStateStatus(ctx, "createNamespaceState", "failed", map[string]interface{}{"error": err.Error()})
		return nil, err
	}
	persistence.SetDataAttribute("nsname", nsname)
	reportStateStatus(ctx, "createNamespaceState", "success", map[string]interface{}{"nsname": nsname})
	return iwf.SingleNextState(&createJobState{svc: i.svc}, input), nil
}

type createJobState struct {
	iwf.WorkflowStateDefaultsNoWaitUntil
	svc service.ClusterCreateService
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
	if err := i.svc.CreateJob(ctx, operation, nsname); err != nil {
		reportStateStatus(ctx, "createJobState", "failed", map[string]interface{}{"error": err.Error()})
		return nil, err
	}
	reportStateStatus(ctx, "createJobState", "success", map[string]interface{}{"nsname": nsname})
	return iwf.SingleNextState(&clusterOperationSuccessfulCheckState{svc: i.svc}, input), nil
}

type clusterOperationSuccessfulCheckState struct {
	iwf.WorkflowStateDefaultsNoWaitUntil
	svc service.ClusterCreateService
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

	if err := i.svc.WaitForClusterOperationToBeCompleted(ctx, nsname); err != nil {
		logger.Error(err, "failed to create cluster")
		persistence.SetDataAttribute("cleanup_reason", "failed")
		reportStateStatus(ctx, "clusterOperationCheck", "failed", map[string]interface{}{"error": err.Error()})
		return iwf.SingleNextState(&cleanupNamespaceState{svc: i.svc}, input), nil
	}

	logger.Info("Successfully Created Cluster")
	persistence.SetDataAttribute("cleanup_reason", "success")
	reportStateStatus(ctx, "clusterOperationCheck", "success", map[string]interface{}{"nsname": nsname})
	return iwf.SingleNextState(&syncCredentialState{svc: i.svc}, input), nil
}

type syncCredentialState struct {
	iwf.WorkflowStateDefaultsNoWaitUntil
	svc service.ClusterCreateService
}

func (i syncCredentialState) Execute(
	ctx iwf.WorkflowContext, input iwf.Object, commandResults iwf.CommandResults, persistence iwf.Persistence,
	communication iwf.Communication,
) (*iwf.StateDecision, error) {
	var nsname string
	persistence.GetDataAttribute("nsname", &nsname)

	var operation common.KubeVirtCreateOperation
	input.Get(&operation)
	kubeconfig := operation.KubeVirtCredential.KubeConfig
	if err := i.svc.SyncCredential(ctx, kubeconfig, operation, nsname); err != nil {
		reportStateStatus(ctx, "syncCredentialState", "failed", map[string]interface{}{"error": err.Error()})
		return nil, fmt.Errorf("failed to sync credential: %v", err)
	}
	reportStateStatus(ctx, "syncCredentialState", "success", map[string]interface{}{"nsname": nsname})
	return iwf.SingleNextState(&cleanupNamespaceState{svc: i.svc}, input), nil
}

type cleanupNamespaceState struct {
	iwf.WorkflowStateDefaultsNoWaitUntil
	svc service.ClusterCreateService
}

func (i cleanupNamespaceState) Execute(
	ctx iwf.WorkflowContext, input iwf.Object, commandResults iwf.CommandResults, persistence iwf.Persistence,
	communication iwf.Communication,
) (*iwf.StateDecision, error) {
	logger := logr.FromContextOrDiscard(ctx)

	var nsname string
	persistence.GetDataAttribute("nsname", &nsname)
	var reason string
	persistence.GetDataAttribute("cleanup_reason", &reason)

	if err := i.svc.CleanupNamespace(ctx, nsname); err != nil {
		logger.Error(err, "failed to cleanup namespace")
		reportStateStatus(ctx, "cleanupNamespaceState", "failed", map[string]interface{}{"error": err.Error()})
		return nil, err
	}
	reportStateStatus(ctx, "cleanupNamespaceState", reason, map[string]interface{}{"nsname": nsname})
	if reason == "failed" {
		return iwf.ForceFailWorkflow("Cluster creation failed, namespace cleaned up."), nil
	}
	return iwf.SingleNextState(&waitForeverState{}, input), nil
}

type waitForeverState struct {
	iwf.WorkflowStateDefaultsNoWaitUntil
}

func (s waitForeverState) Execute(
	ctx iwf.WorkflowContext,
	input iwf.Object,
	commandResults iwf.CommandResults,
	persistence iwf.Persistence,
	communication iwf.Communication,
) (*iwf.StateDecision, error) {
	reportStateStatus(ctx, "waitForeverState", "running", nil)
	for {
		time.Sleep(24 * time.Hour)
	}
	return iwf.GracefulCompletingWorkflow, nil
}

// input object dibo.
// sleep dibo. completion hobena.

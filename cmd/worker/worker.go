package main

import (
	"fmt"
	"github.com/RejwankabirHamim/cadence-iwf-poc/workflows"
	"github.com/gin-gonic/gin"
	"github.com/indeedeng/iwf-golang-sdk/gen/iwfidl"
	"github.com/indeedeng/iwf-golang-sdk/iwf"
	"github.com/urfave/cli"
	"log"
	"net/http"
	"sync"
)

// BuildCLI is the main entry point for the iwf worker
func BuildCLI() *cli.App {
	app := cli.NewApp()
	app.Name = "iwf golang samples"
	app.Usage = "iwf golang samples"
	app.Version = "beta"

	app.Commands = []cli.Command{
		{
			Name:    "start",
			Aliases: []string{""},
			Usage:   "start iwf golang samples",
			Action:  start,
		},
	}
	return app
}

func start(c *cli.Context) {
	fmt.Println("start running samples")
	closeFn := startWorkflowWorker()
	// TODO improve the waiting with process signal
	wg := sync.WaitGroup{}
	wg.Add(1)
	wg.Wait()
	closeFn()
}

var workerService = iwf.NewWorkerService(workflows.GetRegistry(), nil)

func startWorkflowWorker() (closeFunc func()) {
	router := gin.Default()
	router.POST(iwf.WorkflowStateWaitUntilApi, apiV1WorkflowStateStart)
	router.POST(iwf.WorkflowStateExecuteApi, apiV1WorkflowStateDecide)
	router.POST(iwf.WorkflowWorkerRPCAPI, apiV1WorkflowWorkerRpc)

	wfServer := &http.Server{
		Addr:    ":" + iwf.DefaultWorkerPort,
		Handler: router,
	}
	go func() {
		if err := wfServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()
	return func() { wfServer.Close() }
}

func apiV1WorkflowStateStart(c *gin.Context) {
	var req iwfidl.WorkflowStateWaitUntilRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := workerService.HandleWorkflowStateWaitUntil(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
	return
}
func apiV1WorkflowStateDecide(c *gin.Context) {
	var req iwfidl.WorkflowStateExecuteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := workerService.HandleWorkflowStateExecute(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
	return
}

func apiV1WorkflowWorkerRpc(c *gin.Context) {
	var req iwfidl.WorkflowWorkerRpcRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := workerService.HandleWorkflowWorkerRPC(c.Request.Context(), req)
	if err != nil {
		c.JSON(501, iwfidl.WorkerErrorResponse{
			Detail:    iwfidl.PtrString(err.Error()),
			ErrorType: iwfidl.PtrString("test-error-type"),
		})
		return
	}
	c.JSON(http.StatusOK, resp)
	return
}

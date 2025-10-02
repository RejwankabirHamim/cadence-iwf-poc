package persistence

import (
	"sync"
)

// StateStatus represents one state transition in workflow
type StateStatus struct {
	WorkflowID string                 `json:"workflowId"`
	StateName  string                 `json:"stateName"`
	Status     string                 `json:"status"`
	Data       map[string]interface{} `json:"data,omitempty"`
}

var (
	historyStore = make(map[string][]StateStatus)
	mu           sync.RWMutex
)

// Save persists state transition in memory
func Save(workflowID string, state StateStatus) {
	mu.Lock()
	defer mu.Unlock()
	historyStore[workflowID] = append(historyStore[workflowID], state)
}

// Get returns all state transitions for a workflow
func Get(workflowID string) []StateStatus {
	mu.RLock()
	defer mu.RUnlock()
	return historyStore[workflowID]
}

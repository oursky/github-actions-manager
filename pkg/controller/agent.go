package controller

import (
	"time"
)

type AgentState string

const (
	AgentStatePending     AgentState = "pending"
	AgentStateConfiguring AgentState = "configuring"
	AgentStateStarting    AgentState = "starting"
	AgentStateReady       AgentState = "ready"
	AgentStateTerminating AgentState = "terminating"
)

type Agent struct {
	ID                 string     `json:"id"`
	RunnerName         string     `json:"runnerName"`
	State              AgentState `json:"state"`
	LastTransitionTime time.Time  `json:"lastTransitionTime"`
	RunnerID           *int64     `json:"runnerID"`
}

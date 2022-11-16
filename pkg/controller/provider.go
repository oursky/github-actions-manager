package controller

import (
	"context"
	"net/http"

	"github.com/oursky/github-actions-manager/pkg/github/runners"
)

type Capabilities struct {
	KeepAgentsOnExit bool
}

type Provider interface {
	State() State
	Shutdown()
	Capabilities() Capabilities
	AuthenticateRequest(rw http.ResponseWriter, r *http.Request, next http.Handler)
	RegisterAgent(r *http.Request, hostName string, regToken string, targetURL string, disableUpdate bool) (*AgentResponse, error)
	CheckAgent(ctx context.Context, agent *Agent, runner *runners.Instance) error
	TerminateAgent(ctx context.Context, agent Agent) error
}

package controller

import (
	"context"
	"net/http"

	"github.com/oursky/github-actions-manager/pkg/github/runners"
)

type Provider interface {
	State() State
	Shutdown()
	AuthenticateRequest(rw http.ResponseWriter, r *http.Request, next http.Handler)
	RegisterAgent(r *http.Request, runnerName string, regToken string, targetURL string) (*AgentResponse, error)
	CheckAgent(ctx context.Context, agent *Agent, runner *runners.Instance) error
	TerminateAgent(ctx context.Context, agent Agent) error
}

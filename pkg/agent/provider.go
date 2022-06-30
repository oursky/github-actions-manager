package agent

import (
	"context"
	"io"
	"net/http"

	"github.com/oursky/github-actions-manager/pkg/controller"
)

type Provider interface {
	Shutdown(ctx context.Context)
	NewControllerRequest(ctx context.Context, method string, path string, body io.Reader) (*http.Request, error)
	OnAgentRegistered(controller.Agent)
}

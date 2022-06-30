package kube

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"

	"github.com/oursky/github-actions-manager/pkg/controller"

	"golang.org/x/sync/errgroup"
)

type AgentProvider struct {
	url        url.URL
	tokenPath  string
	shouldHalt bool
}

func NewAgentProvider(controllerURL string, tokenPath string) (*AgentProvider, error) {
	url, err := url.Parse(controllerURL)
	if err != nil {
		return nil, err
	}

	return &AgentProvider{
		url:       *url,
		tokenPath: tokenPath,
	}, nil
}

func (p *AgentProvider) Start(ctx context.Context, g *errgroup.Group) error {
	return nil
}

func (p *AgentProvider) Shutdown(ctx context.Context) {
	if p.shouldHalt {
		<-ctx.Done()
	}
}

func (p *AgentProvider) OnAgentRegistered(agent controller.Agent) {
	// registered with controller now; let controller delete the agent pod
	p.shouldHalt = true
}

func (p *AgentProvider) NewControllerRequest(ctx context.Context, method string, urlPath string, body io.Reader) (*http.Request, error) {
	url := p.url
	url.Path = path.Join(url.Path, urlPath)

	r, err := http.NewRequestWithContext(ctx, method, url.String(), body)
	if err != nil {
		return nil, err
	}

	token, err := os.ReadFile(p.tokenPath)
	if err != nil {
		return nil, err
	}

	r.Header.Set("Authorization", "Bearer "+string(token))
	return r, nil
}

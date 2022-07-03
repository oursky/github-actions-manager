package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"time"

	"github.com/oursky/github-actions-manager/pkg/controller"
	"github.com/oursky/github-actions-manager/pkg/utils/httputil"
)

type controllerAPI struct {
	client   *http.Client
	provider Provider
}

func newControllerAPI(provider Provider) *controllerAPI {
	return &controllerAPI{
		client:   &http.Client{Timeout: 10 * time.Second},
		provider: provider,
	}
}

func (c *controllerAPI) doJSON(r *http.Request, result any) error {
	resp, err := c.client.Do(r)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if err := httputil.CheckStatus(resp); err != nil {
		return err
	}

	return json.NewDecoder(resp.Body).Decode(&result)
}

func (c *controllerAPI) RegisterAgent(ctx context.Context, hostName string) (*controller.AgentResponse, error) {
	r, err := c.provider.NewControllerRequest(
		ctx,
		http.MethodPost,
		"api/v1/agent",
		bytes.NewBufferString(url.Values{"hostName": []string{hostName}}.Encode()),
	)
	if err != nil {
		return nil, err
	}

	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp := &controller.AgentResponse{}
	if err := c.doJSON(r, &resp); err != nil {
		return nil, err
	}

	return resp, nil
}

func (c *controllerAPI) GetAgent(ctx context.Context, id string) (name string, state controller.AgentState, err error) {
	r, err := c.provider.NewControllerRequest(
		ctx,
		http.MethodGet,
		"api/v1/agent/"+url.PathEscape(id),
		nil,
	)
	if err != nil {
		return "", "", err
	}

	resp := &struct {
		RunnerName string                `json:"runnerName"`
		State      controller.AgentState `json:"state"`
	}{}
	if err := c.doJSON(r, &resp); err != nil {
		return "", "", err
	}

	return resp.RunnerName, resp.State, nil
}

func (c *controllerAPI) DeleteAgent(ctx context.Context, id string) error {
	r, err := c.provider.NewControllerRequest(
		ctx,
		http.MethodDelete,
		"api/v1/agent/"+url.PathEscape(id),
		nil,
	)
	if err != nil {
		return err
	}

	resp, err := c.client.Do(r)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return httputil.CheckStatus(resp)
}

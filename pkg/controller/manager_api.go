package controller

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"time"

	"github.com/oursky/github-actions-manager/pkg/github/runners"
	"github.com/oursky/github-actions-manager/pkg/utils/httputil"
)

type managerAPI struct {
	client *http.Client
	base   url.URL
	key    string
}

func newManagerAPI(config *Config) *managerAPI {
	url, err := url.Parse(config.ManagerURL)
	if err != nil {
		panic(err)
	}
	return &managerAPI{
		client: &http.Client{Timeout: 10 * time.Second},
		base:   *url,
		key:    config.ManagerAuthKey,
	}
}

func (m *managerAPI) doJSON(r *http.Request, result any) error {
	resp, err := m.client.Do(r)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if err := httputil.CheckStatus(resp); err != nil {
		return err
	}

	return json.NewDecoder(resp.Body).Decode(&result)
}

func (m *managerAPI) GetRegistrationToken(ctx context.Context) (token string, targetURL string, err error) {
	url := m.base
	url.Path = path.Join(url.Path, "api/v1/token")

	r, err := http.NewRequestWithContext(ctx, "GET", url.String(), nil)
	r.Header.Add("Authorization", "Bearer "+m.key)
	if err != nil {
		return "", "", err
	}

	var resp struct {
		Token string `json:"token"`
		URL   string `json:"url"`
	}
	if err := m.doJSON(r, &resp); err != nil {
		return "", "", err
	}

	return resp.Token, resp.URL, nil
}

func (m *managerAPI) GetRunners(ctx context.Context) (epoch int64, instances map[string]runners.Instance, err error) {
	url := m.base
	url.Path = path.Join(url.Path, "api/v1/runners")

	r, err := http.NewRequestWithContext(ctx, "GET", url.String(), nil)
	r.Header.Add("Authorization", "Bearer "+m.key)
	if err != nil {
		return 0, nil, err
	}

	var resp struct {
		Epoch   int64
		Runners []runners.Instance
	}
	if err := m.doJSON(r, &resp); err != nil {
		return 0, nil, err
	}

	epoch = resp.Epoch
	instances = make(map[string]runners.Instance)
	for _, r := range resp.Runners {
		instances[r.Name] = r
	}
	return epoch, instances, nil
}

func (m *managerAPI) DeleteRunner(ctx context.Context, id int64) error {
	url := m.base
	url.Path = path.Join(url.Path, "api/v1/runners/"+strconv.FormatInt(id, 10))

	r, err := http.NewRequestWithContext(ctx, "DELETE", url.String(), nil)
	r.Header.Add("Authorization", "Bearer "+m.key)
	if err != nil {
		return err
	}

	resp, err := m.client.Do(r)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return httputil.CheckStatus(resp)
}

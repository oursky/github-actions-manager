package webhook

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/go-github/v45/github"
	"github.com/oursky/github-actions-manager/pkg/github/auth"
)

type Source interface {
	listDeliveries(ctx context.Context, cursor string, pageSize int) ([]*github.HookDelivery, string, error)
	getDelivery(ctx context.Context, id int64) (*github.HookDelivery, error)
}

func NewSource(http *http.Client) (Source, error) {
	authTr, ok := http.Transport.(auth.AppTransport)
	if !ok {
		return nil, fmt.Errorf("unsupported transport: %T", http.Transport)
	}

	client := *http
	client.Transport = authTr.AppsTransport
	return &SourceApp{client: github.NewClient(&client)}, nil
}

type SourceApp struct {
	client *github.Client
}

func (s *SourceApp) listDeliveries(ctx context.Context, cursor string, pageSize int) ([]*github.HookDelivery, string, error) {
	deliveries, resp, err := s.client.Apps.ListHookDeliveries(ctx, &github.ListCursorOptions{
		Cursor:  cursor,
		PerPage: pageSize,
	})
	if err != nil {
		return nil, "", err
	}

	return deliveries, resp.Cursor, nil
}
func (s *SourceApp) getDelivery(ctx context.Context, id int64) (*github.HookDelivery, error) {
	delivery, _, err := s.client.Apps.GetHookDelivery(ctx, id)
	return delivery, err
}

package github

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/go-github/v45/github"
)

type TargetRepository struct {
	client *github.Client

	Name  string
	Owner string
}

func NewTargetRepository(http *http.Client, name string, owner string) *TargetRepository {
	client := github.NewClient(http)
	return &TargetRepository{client: client, Name: name, Owner: owner}
}

func (t *TargetRepository) URL() string {
	return fmt.Sprintf("https://github.com/%s/%s", t.Owner, t.Name)
}

func (t *TargetRepository) GetRegistrationToken(ctx context.Context) (*github.RegistrationToken, error) {
	token, _, err := t.client.Actions.CreateRegistrationToken(ctx, t.Owner, t.Name)
	if err != nil {
		return nil, err
	}

	return token, nil
}

func (t *TargetRepository) GetRunners(
	ctx context.Context, page int, pageSize int,
) ([]*github.Runner, int, error) {
	runners, resp, err := t.client.Actions.ListRunners(
		ctx, t.Owner, t.Name,
		&github.ListOptions{Page: page, PerPage: pageSize},
	)
	if err != nil {
		return nil, 0, err
	}

	return runners.Runners, resp.NextPage, nil
}

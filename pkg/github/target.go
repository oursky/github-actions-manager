package github

import (
	"context"
	"fmt"
	"net/http"
	"regexp"

	"github.com/google/go-github/v45/github"
)

type Target interface {
	URL() string
	GetRunners(ctx context.Context, page int, pageSize int) (runners []*github.Runner, nextPage int, err error)
}

var (
	regexTargetRepo = regexp.MustCompile(`https://github\.com/([^/]+)/([^/]+)/?`)
	regexTargetOrg  = regexp.MustCompile(`https://github\.com/([^/]+)/?`)
)

func NewTarget(http *http.Client, url string) (Target, error) {
	if match := regexTargetRepo.FindStringSubmatch(url); match != nil {
		owner := match[1]
		name := match[2]
		return NewTargetRepository(http, name, owner), nil
	}

	if match := regexTargetOrg.FindStringSubmatch(url); match != nil {
		name := match[1]
		return NewTargetOrganization(http, name), nil
	}

	return nil, fmt.Errorf("unsupported GitHub target URL: %s", url)
}

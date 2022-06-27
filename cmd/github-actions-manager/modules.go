package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/oursky/github-actions-manager/pkg/cmd"
	"github.com/oursky/github-actions-manager/pkg/dashboard"
	"github.com/oursky/github-actions-manager/pkg/github"
	"github.com/oursky/github-actions-manager/pkg/github/auth"
	"github.com/oursky/github-actions-manager/pkg/github/jobs"
	"github.com/oursky/github-actions-manager/pkg/github/runners"
	"github.com/oursky/github-actions-manager/pkg/utils/defaults"
	"github.com/oursky/github-actions-manager/pkg/utils/ratelimit"
	"golang.org/x/time/rate"

	"go.uber.org/zap"
)

func initModules(logger *zap.Logger, config *Config) ([]cmd.Module, error) {
	transport, err := auth.NewTransport(
		&config.GitHub.Auth,
		ratelimit.NewTransport(
			http.DefaultTransport,
			rate.Limit(defaults.Value(config.GitHub.RPS, 1)),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("cannot setup GitHub client: %w", err)
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   defaults.Value(config.GitHub.HTTPTimeout.Value(), 10*time.Second),
	}

	target, err := github.NewTarget(client, config.GitHub.TargetURL)
	if err != nil {
		return nil, fmt.Errorf("cannot setup GitHub target: %w", err)
	}

	var modules []cmd.Module

	runners := runners.NewSynchronizer(logger, &config.GitHub.Runners, target)
	//modules = append(modules, sync)

	jobs, err := jobs.NewSynchronizer(logger, &config.GitHub.Jobs, client)
	if err != nil {
		return nil, fmt.Errorf("cannot setup job sync: %w", err)
	}
	modules = append(modules, jobs)

	dashboard := dashboard.NewServer(logger, &config.Dashboard, runners)
	modules = append(modules, dashboard)

	return modules, nil
}

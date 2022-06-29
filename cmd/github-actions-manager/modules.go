package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/oursky/github-actions-manager/pkg/api"
	"github.com/oursky/github-actions-manager/pkg/cmd"
	"github.com/oursky/github-actions-manager/pkg/dashboard"
	"github.com/oursky/github-actions-manager/pkg/github"
	"github.com/oursky/github-actions-manager/pkg/github/auth"
	"github.com/oursky/github-actions-manager/pkg/github/jobs"
	"github.com/oursky/github-actions-manager/pkg/github/runners"
	"github.com/oursky/github-actions-manager/pkg/kv"
	"github.com/oursky/github-actions-manager/pkg/slack"
	"github.com/oursky/github-actions-manager/pkg/utils/defaults"
	"github.com/oursky/github-actions-manager/pkg/utils/ratelimit"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"

	gh "github.com/google/go-github/v45/github"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

func initModules(logger *zap.Logger, config *Config) ([]cmd.Module, error) {
	registry := prometheus.NewPedanticRegistry()
	registry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	registry.MustRegister(collectors.NewGoCollector())

	transport, err := auth.NewTransport(
		&config.GitHub.Auth,
		ratelimit.NewTransport(
			http.DefaultTransport,
			rate.Limit(defaults.Value(config.GitHub.RPS, 1)),
			defaults.Value(config.GitHub.Brust, 60),
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

	runners := runners.NewSynchronizer(logger, &config.GitHub.Runners, target, registry)
	modules = append(modules, runners)

	jobs, err := jobs.NewSynchronizer(logger, &config.GitHub.Jobs, client, registry)
	if err != nil {
		return nil, fmt.Errorf("cannot setup job sync: %w", err)
	}
	modules = append(modules, jobs)

	if !config.Slack.Disabled {
		kv, err := kv.NewKubeConfigMapStore(logger, config.Store.KubeNamespace)
		if err != nil {
			return nil, fmt.Errorf("cannot setup store: %w", err)
		}
		modules = append(modules, kv)

		slackApp := slack.NewApp(logger, &config.Slack, kv)
		modules = append(modules, slackApp)

		notifier := slack.NewNotifier(logger, slackApp, gh.NewClient(client), jobs)
		modules = append(modules, notifier)
	}

	dashboard := dashboard.NewServer(logger, &config.Dashboard, runners, jobs)
	modules = append(modules, dashboard)

	api := api.NewServer(logger, &config.API, runners, target, registry)
	modules = append(modules, api)

	return modules, nil
}

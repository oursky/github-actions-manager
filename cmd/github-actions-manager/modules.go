package main

import (
	"fmt"
	"time"

	"github.com/oursky/github-actions-manager/pkg/cmd"
	"github.com/oursky/github-actions-manager/pkg/github"
	"github.com/oursky/github-actions-manager/pkg/github/auth"
	"github.com/oursky/github-actions-manager/pkg/github/runner"
	"github.com/oursky/github-actions-manager/pkg/utils/defaults"

	"go.uber.org/zap"
)

func initModules(logger *zap.Logger, config *Config) ([]cmd.Module, error) {
	client, err := auth.NewClient(&config.GitHub.Auth)
	if err != nil {
		return nil, fmt.Errorf("cannot setup GitHub client: %w", err)
	}

	client.Timeout = defaults.Value(config.GitHub.HTTPTimeout.Value(), 10*time.Second)

	target, err := github.NewTarget(client, config.GitHub.TargetURL)
	if err != nil {
		return nil, fmt.Errorf("cannot setup GitHub target: %w", err)
	}

	var modules []cmd.Module

	sync := runner.NewSynchronizer(logger, &config.GitHub.Runners, target)
	modules = append(modules, sync)

	return modules, nil
}

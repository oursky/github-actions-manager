package main

import (
	"fmt"

	"github.com/oursky/github-actions-manager/pkg/agent"
	"github.com/oursky/github-actions-manager/pkg/cmd"
	"github.com/oursky/github-actions-manager/pkg/kube"

	"go.uber.org/zap"
)

func initModules(logger *zap.Logger, config *Config) ([]cmd.Module, error) {
	var modules []cmd.Module

	provider, err := kube.NewAgentProvider(config.ControllerURL, config.TokenPath)
	if err != nil {
		return nil, fmt.Errorf("failed to init agent: %w", err)
	}
	modules = append(modules, provider)

	agent := agent.NewAgent(logger, &config.Agent, provider)
	modules = append(modules, agent)

	return modules, nil
}

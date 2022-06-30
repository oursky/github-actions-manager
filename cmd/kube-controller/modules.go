package main

import (
	"fmt"

	"github.com/oursky/github-actions-manager/pkg/cmd"
	"github.com/oursky/github-actions-manager/pkg/controller"
	"github.com/oursky/github-actions-manager/pkg/kube"

	"go.uber.org/zap"
)

func initModules(logger *zap.Logger, config *Config) ([]cmd.Module, error) {
	var modules []cmd.Module

	provider, err := kube.NewControllerProvider(logger)
	if err != nil {
		return nil, fmt.Errorf("failed to init controller: %w", err)
	}
	modules = append(modules, provider)

	controller := controller.NewController(logger, &config.Controller, provider)
	modules = append(modules, controller)

	return modules, nil
}

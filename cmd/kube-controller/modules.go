package main

import (
	"fmt"

	"github.com/oursky/github-actions-manager/pkg/cmd"
	"github.com/oursky/github-actions-manager/pkg/controller"
	"github.com/oursky/github-actions-manager/pkg/kube"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"

	"go.uber.org/zap"
)

func initModules(logger *zap.Logger, config *Config) ([]cmd.Module, error) {
	registry := prometheus.NewPedanticRegistry()
	registry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	registry.MustRegister(collectors.NewGoCollector())

	var modules []cmd.Module

	provider, err := kube.NewControllerProvider(logger, registry)
	if err != nil {
		return nil, fmt.Errorf("failed to init controller: %w", err)
	}
	modules = append(modules, provider)

	controller := controller.NewController(logger, &config.Controller, registry, provider)
	modules = append(modules, controller)

	return modules, nil
}

package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/oursky/github-actions-manager/pkg/api"
	"github.com/oursky/github-actions-manager/pkg/dashboard"
	"github.com/oursky/github-actions-manager/pkg/github/auth"
	"github.com/oursky/github-actions-manager/pkg/github/jobs"
	"github.com/oursky/github-actions-manager/pkg/github/runners"
	"github.com/oursky/github-actions-manager/pkg/kv"
	"github.com/oursky/github-actions-manager/pkg/slack"

	"github.com/go-playground/validator/v10"
	"github.com/spf13/viper"
)

type Config struct {
	GitHub    GitHubConfig
	Dashboard dashboard.Config
	Store     kv.Config
	Slack     slack.Config
	API       api.Config
}

type GitHubConfig struct {
	TargetURL   string `validate:"required,url"`
	RPS         *float64
	Brust       *int
	HTTPTimeout *time.Duration
	Auth        auth.Config
	Runners     runners.Config
	Jobs        jobs.Config
}

type StoreConfig struct {
	KubeNamespace string `validate:"required"`
}

func NewConfig(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("toml")
	v.SetEnvPrefix("GHA_MANAGER")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	v.SetTypeByDefaultValue(true)

	if path != "" {
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("failed to decode config: %w", err)
		}
	}

	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	validate := validator.New()
	if err := validate.Struct(config); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &config, nil
}

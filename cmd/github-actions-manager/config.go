package main

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/oursky/github-actions-manager/pkg/api"
	"github.com/oursky/github-actions-manager/pkg/dashboard"
	"github.com/oursky/github-actions-manager/pkg/github/auth"
	"github.com/oursky/github-actions-manager/pkg/github/jobs"
	"github.com/oursky/github-actions-manager/pkg/github/runners"
	"github.com/oursky/github-actions-manager/pkg/kv"
	"github.com/oursky/github-actions-manager/pkg/slack"
	"github.com/oursky/github-actions-manager/pkg/utils/tomltypes"

	"github.com/BurntSushi/toml"
	"github.com/go-playground/validator/v10"
)

type Config struct {
	GitHub    GitHubConfig     `toml:"github"`
	Dashboard dashboard.Config `toml:"dashboard"`
	Store     kv.Config        `toml:"store"`
	Slack     slack.Config     `toml:"slack"`
	API       api.Config       `toml:"api"`
}

type GitHubConfig struct {
	TargetURL   string              `toml:"targetURL" validate:"required,url"`
	RPS         *float64            `toml:"rps,omitempty"`
	Brust       *int                `toml:"brust,omitempty"`
	HTTPTimeout *tomltypes.Duration `toml:"httpTimeout,omitempty"`
	Auth        auth.Config         `toml:"auth"`
	Runners     runners.Config      `toml:"runners,omitempty"`
	Jobs        jobs.Config         `toml:"jobs,omitempty"`
}

type StoreConfig struct {
	KubeNamespace string `toml:"kubeNamespace,omitempty" validate:"required"`
}

func NewConfig(path string) (*Config, error) {
	var config Config
	if _, err := toml.DecodeFile(path, &config); err != nil {
		return nil, fmt.Errorf("failed to decode config: %w", err)
	}

	validate := validator.New()
	validate.RegisterTagNameFunc(func(f reflect.StructField) string {
		name, _, _ := strings.Cut(f.Tag.Get("toml"), ",")
		if name == "-" {
			return ""
		}
		return name
	})

	if err := validate.Struct(config); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &config, nil
}

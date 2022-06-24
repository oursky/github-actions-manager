package main

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/oursky/github-actions-manager/pkg/github/auth"
	"github.com/oursky/github-actions-manager/pkg/github/runner"
	"github.com/oursky/github-actions-manager/pkg/utils/tomltypes"

	"github.com/BurntSushi/toml"
	"github.com/go-playground/validator/v10"
)

type Config struct {
	GitHub GitHubConfig `toml:"github"`
}

type GitHubConfig struct {
	TargetURL   string              `toml:"targetURL" validate:"required,url"`
	HTTPTimeout *tomltypes.Duration `toml:"httpTimeout,omitempty"`
	Auth        auth.Config         `toml:"auth"`
	Runners     runner.Config       `toml:"runners"`
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

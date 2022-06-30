package main

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/oursky/github-actions-manager/pkg/controller"

	"github.com/BurntSushi/toml"
	"github.com/go-playground/validator/v10"
)

type Config struct {
	Controller controller.Config
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

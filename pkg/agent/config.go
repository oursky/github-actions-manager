package agent

import (
	"time"

	"github.com/oursky/github-actions-manager/pkg/utils/defaults"
)

type Config struct {
	RunnerDir       string `validate:"required,dir"`
	WorkDir         string `validate:"required"`
	ConfigureScript *string
	RunScript       *string
	WatchInterval   *time.Duration
}

func (c *Config) GetWatchInterval() time.Duration {
	return defaults.Value(c.WatchInterval, 5*time.Second)
}

func (c *Config) GetConfigureScript() string {
	return defaults.Value(c.ConfigureScript, "./config.sh")
}

func (c *Config) GetRunScript() string {
	return defaults.Value(c.RunScript, "./run.sh")
}

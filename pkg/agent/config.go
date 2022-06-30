package agent

import (
	"time"

	"github.com/oursky/github-actions-manager/pkg/utils/defaults"
	"github.com/oursky/github-actions-manager/pkg/utils/tomltypes"
)

type Config struct {
	RunnerDir       string              `toml:"runnerDir" validate:"required,dir"`
	WorkDir         string              `toml:"workDir" validate:"required,dir"`
	ConfigureScript *string             `toml:"configureScript"`
	RunScript       *string             `toml:"runScript"`
	WatchInterval   *tomltypes.Duration `toml:"watchInterval,omitempty"`
}

func (c *Config) GetWatchInterval() time.Duration {
	return defaults.Value(c.WatchInterval.Value(), 5*time.Second)
}

func (c *Config) GetConfigureScript() string {
	return defaults.Value(c.ConfigureScript, "./config.sh")
}

func (c *Config) GetRunScript() string {
	return defaults.Value(c.RunScript, "./run.sh")
}

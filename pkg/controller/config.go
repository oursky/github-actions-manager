package controller

import (
	"time"

	"github.com/oursky/github-actions-manager/pkg/utils/defaults"
	"github.com/oursky/github-actions-manager/pkg/utils/tomltypes"
)

type Config struct {
	ManagerURL        string              `toml:"managerURL" validate:"required,url"`
	ManagerAuthKey    string              `toml:"managerAuthKey" validate:"required"`
	Addr              *string             `toml:"addr,omitempty" validate:"omitempty,tcp_addr"`
	SyncInterval      *tomltypes.Duration `toml:"syncInterval,omitempty"`
	TransitionTimeout *tomltypes.Duration `toml:"transitionTimeout,omitempty"`
}

func (c *Config) GetSyncInterval() time.Duration {
	return defaults.Value(c.SyncInterval.Value(), 5*time.Second)
}

func (c *Config) GetTransitionTimeout() time.Duration {
	return defaults.Value(c.TransitionTimeout.Value(), 1*time.Minute)
}

func (c *Config) GetAddr() string {
	return defaults.Value(c.Addr, "127.0.0.1:8007")
}

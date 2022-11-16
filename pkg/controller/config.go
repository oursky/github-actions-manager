package controller

import (
	"time"

	"github.com/oursky/github-actions-manager/pkg/utils/defaults"
)

type Config struct {
	ManagerURL        string  `validate:"required,url"`
	ManagerAuthKey    string  `validate:"required"`
	Addr              *string `validate:"omitempty,tcp_addr"`
	DisableUpdate     *bool
	SyncInterval      *time.Duration
	TransitionTimeout *time.Duration
}

// FIXME: configure it at manager instead of controller
func (c *Config) GetDisableUpdate() bool {
	return defaults.Value(c.DisableUpdate, false)
}

func (c *Config) GetSyncInterval() time.Duration {
	return defaults.Value(c.SyncInterval, 5*time.Second)
}

func (c *Config) GetTransitionTimeout() time.Duration {
	return defaults.Value(c.TransitionTimeout, 1*time.Minute)
}

func (c *Config) GetAddr() string {
	return defaults.Value(c.Addr, "127.0.0.1:8007")
}

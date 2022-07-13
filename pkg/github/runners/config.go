package runners

import (
	"time"

	"github.com/oursky/github-actions-manager/pkg/utils/defaults"
)

type Config struct {
	SyncInterval *time.Duration
	SyncPageSize *int `validate:"omitempty,min=1,max=100"`
}

func (c *Config) GetSyncInterval() time.Duration {
	return defaults.Value(c.SyncInterval, 5*time.Second)
}

func (c *Config) GetSyncPageSize() int {
	return defaults.Value(c.SyncPageSize, 100)
}

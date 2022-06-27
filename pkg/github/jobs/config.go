package jobs

import (
	"time"

	"github.com/oursky/github-actions-manager/pkg/utils/defaults"
	"github.com/oursky/github-actions-manager/pkg/utils/tomltypes"
)

type Config struct {
	RetentionPeriod *tomltypes.Duration `toml:"retention_period,omitempty"`
	SyncInterval    *tomltypes.Duration `toml:"syncInterval,omitempty"`
	SyncPageSize    *int                `toml:"syncPageSize,omitempty" validate:"omitempty,min=1,max=100"`
}

func (c *Config) GetRetentionPeriod() time.Duration {
	return defaults.Value(c.RetentionPeriod.Value(), 8*time.Hour)
}

func (c *Config) GetSyncInterval() time.Duration {
	return defaults.Value(c.SyncInterval.Value(), 10*time.Second)
}

func (c *Config) GetSyncPageSize() int {
	return defaults.Value(c.SyncPageSize, 30)
}

package jobs

import (
	"time"

	"github.com/oursky/github-actions-manager/pkg/utils/defaults"
)

const KVKey = "jobs"

type Config struct {
	Disabled          bool
	RetentionPeriod   *time.Duration
	SyncInterval      *time.Duration
	SyncPageSize      *int    `validate:"omitempty,min=1,max=100"`
	WebhookServerAddr *string `validate:"omitempty,tcp_addr"`
	WebhookSecret     string  `validate:"required_if=Disabled false"`
}

func (c *Config) GetRetentionPeriod() time.Duration {
	return defaults.Value(c.RetentionPeriod, 1*time.Hour)
}

func (c *Config) GetSyncInterval() time.Duration {
	return defaults.Value(c.SyncInterval, 10*time.Second)
}

func (c *Config) GetSyncPageSize() int {
	return defaults.Value(c.SyncPageSize, 30)
}

func (c *Config) GetWebhookServerAddr() string {
	return defaults.Value(c.WebhookServerAddr, "127.0.0.1:8001")
}

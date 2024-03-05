package slack

import (
	"github.com/oursky/github-actions-manager/pkg/kv"
	"github.com/oursky/github-actions-manager/pkg/utils/defaults"
)

type Config struct {
	Disabled    bool
	BotToken    string `validate:"required_if=Disabled false"`
	AppToken    string `validate:"required_if=Disabled false"`
	CommandName *string
}

func (c *Config) GetCommandName() string {
	return defaults.Value(c.CommandName, "gha")
}

var kvNamespace = kv.RegisterNamespace("slack-subscriptions")

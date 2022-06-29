package slack

import "github.com/oursky/github-actions-manager/pkg/kv"

type Config struct {
	Disabled bool   `toml:"disabled"`
	BotToken string `toml:"botToken" validate:"required_if=Disabled false"`
	AppToken string `toml:"appToken" validate:"required_if=Disabled false"`
}

var kvNamespace = kv.RegisterNamespace("slack-subscriptions")

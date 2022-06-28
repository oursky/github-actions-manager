package slack

import "github.com/oursky/github-actions-manager/pkg/kv"

type Config struct {
	BotToken string `toml:"botToken" validate:"required"`
	AppToken string `toml:"appToken" validate:"required"`
}

var kvNamespace = kv.RegisterNamespace("slack-subscriptions")
